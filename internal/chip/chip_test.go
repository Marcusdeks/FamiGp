package chip

import (
	"bytes"
	"encoding/binary"
	"math"
	"slices"
	"testing"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

// testSong crea una canción de un compás a 120 BPM con las pistas indicadas.
func testSong(tracks ...song.Track) *song.Song {
	return &song.Song{Tempos: []song.Tempo{{Tick: 0, BPM: 120}}, Length: 4 * song.PPQ, Tracks: tracks}
}

func track(name string, percussion bool, pitches ...int) song.Track {
	t := song.Track{Name: name, Percussion: percussion}
	for i, p := range pitches {
		t.Notes = append(t.Notes, song.Note{Start: i * song.PPQ, Duration: song.PPQ, Pitch: p, Velocity: 100})
	}
	return t
}

func nes(t *testing.T) Preset {
	t.Helper()
	p, ok := Find("nes")
	if !ok {
		t.Fatal("falta el preset nes")
	}
	return p
}

func TestDefaultMapping(t *testing.T) {
	s := testSong(
		track("Solo", false, 76, 79),
		track("Batería", true, 36, 38),
		track("Bajo", false, 40, 43),
		track("Vacía", false),
		track("Ritmo", false, 60, 64),
	)
	// Pulsos para solo y ritmo por orden, triángulo para el bajo, ruido para la batería.
	if got, want := DefaultMapping(s, nes(t)), []int{0, 4, 2, 1}; !slices.Equal(got, want) {
		t.Errorf("asignación = %v, se esperaba %v", got, want)
	}
	if got, want := DefaultMapping(testSong(track("Única", false, 60)), nes(t)), []int{0, -1, -1, -1}; !slices.Equal(got, want) {
		t.Errorf("con una sola pista, asignación = %v, se esperaba %v", got, want)
	}
}

// El editor conserva las líneas al cambiar de chip porque todos los presets
// tienen tres voces tonales seguidas de una de ruido.
func TestPresetsShareLayout(t *testing.T) {
	for _, p := range Presets {
		var drums []bool
		for _, v := range p.Voices {
			drums = append(drums, v.Drums())
		}
		if !slices.Equal(drums, []bool{false, false, false, true}) {
			t.Errorf("%s: disposición de voces %v", p.ID, drums)
		}
	}
}

// arrange reparte las pistas de la canción entre las voces según mapping.
func arrange(s *song.Song, p Preset, mapping []int) []song.Track {
	parts := make([]song.Track, len(p.Voices))
	for i, t := range mapping {
		if t >= 0 {
			parts[i] = s.Tracks[t]
		}
	}
	return parts
}

func TestRenderEveryPreset(t *testing.T) {
	s := testSong(track("Acordes", false, 60, 64, 67), track("Bajo", false, 36), track("Batería", true, 36, 42, 38, 46))
	// Un acorde para comprobar el arpegio.
	s.Tracks[0].Notes = append(s.Tracks[0].Notes, song.Note{Start: 3 * song.PPQ, Duration: song.PPQ, Pitch: 72, Velocity: 100})
	s.Tracks[0].Notes = append(s.Tracks[0].Notes, song.Note{Start: 3 * song.PPQ, Duration: song.PPQ, Pitch: 76, Velocity: 100})
	for _, p := range Presets {
		t.Run(p.ID, func(t *testing.T) {
			out, err := Render(s, p, arrange(s, p, DefaultMapping(s, p)))
			if err != nil {
				t.Fatal(err)
			}
			if want := int((2 + tail) * SampleRate); len(out) != want {
				t.Errorf("%d muestras, se esperaban %d", len(out), want)
			}
			peak := 0.0
			for _, x := range out {
				if math.IsNaN(float64(x)) {
					t.Fatal("el audio contiene NaN")
				}
				peak = max(peak, math.Abs(float64(x)))
			}
			if peak < 0.85 || peak > 0.91 {
				t.Errorf("pico = %.3f, se esperaba normalizado a 0,9", peak)
			}
		})
	}
}

func TestRenderSilence(t *testing.T) {
	out, err := Render(testSong(), nes(t), make([]song.Track, 4))
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(out, func(x float32) bool { return x != 0 }) {
		t.Error("una canción sin notas debería producir silencio")
	}
}

func TestRenderTooLong(t *testing.T) {
	s := testSong()
	s.Length = (MaxSeconds + 1) * 2 * song.PPQ // a 120 BPM, dos negras por segundo
	if _, err := Render(s, nes(t), make([]song.Track, 4)); err != ErrTooLong {
		t.Errorf("error = %v, se esperaba ErrTooLong", err)
	}
}

func TestEnvelope(t *testing.T) {
	v := Voice{Attack: 0.1, Decay: 0.1, Sustain: 0.5, Release: 0.2}
	tests := []struct{ t, released, want float64 }{
		{0.05, -1, 0.5}, // mitad del ataque
		{0.15, -1, 0.75},
		{1, -1, 0.5},   // sostenimiento
		{1.1, 1, 0.25}, // mitad del release
		{2, 1, 0},
	}
	for _, tt := range tests {
		if got := v.envelope(tt.t, tt.released); math.Abs(got-tt.want) > 1e-9 {
			t.Errorf("envelope(%v, %v) = %v, se esperaba %v", tt.t, tt.released, got, tt.want)
		}
	}
}

func TestTable4(t *testing.T) {
	table := table4(func(x float64) float64 { return math.Sin(2 * math.Pi * x) })
	for _, v := range table {
		if v < -1 || v > 1 || math.Abs(math.Round((v+1)*7.5)-(v+1)*7.5) > 1e-9 {
			t.Fatalf("valor %v fuera de los 16 niveles de 4 bits", v)
		}
	}
}

func TestWriteWAV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWAV(&buf, []float32{0, 1, -1, 2}); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	le := binary.LittleEndian
	if string(b[:4]) != "RIFF" || string(b[8:16]) != "WAVEfmt " || string(b[36:40]) != "data" {
		t.Fatalf("cabecera inválida: %q", b[:44])
	}
	if le.Uint32(b[24:]) != SampleRate || le.Uint32(b[40:]) != 8 || len(b) != 52 {
		t.Errorf("tamaños o frecuencia incorrectos: %v", b[:44])
	}
	// Las muestras fuera de rango se recortan en lugar de desbordarse.
	samples := []int16{0, math.MaxInt16, -math.MaxInt16, math.MaxInt16}
	for i, want := range samples {
		if got := int16(le.Uint16(b[44+2*i:])); got != want {
			t.Errorf("muestra %d = %d, se esperaba %d", i, got, want)
		}
	}
}

// zeroCrossings cuenta los cambios de signo por segundo: una medida sencilla
// de lo grave (pocos) o brillante (muchos) que es un sonido.
func zeroCrossings(samples []float32) float64 {
	n := 0
	for i := 1; i < len(samples); i++ {
		if (samples[i-1] < 0) != (samples[i] < 0) {
			n++
		}
	}
	return float64(n) * SampleRate / float64(len(samples))
}

func TestPreviewDrumsSoundDifferent(t *testing.T) {
	sound := func(pitch int) []float32 {
		out, err := Preview(nes(t), 3, pitch)
		if err != nil || len(out) == 0 {
			t.Fatalf("Preview(%d) = %d muestras, %v", pitch, len(out), err)
		}
		return out
	}
	kick, snare, hat := zeroCrossings(sound(36)), zeroCrossings(sound(38)), zeroCrossings(sound(42))
	if !(kick < snare && snare < hat) {
		t.Errorf("bombo %.0f, caja %.0f, charles %.0f cruces/s: deberían ir de grave a brillante", kick, snare, hat)
	}
	if _, err := Preview(nes(t), 9, 60); err == nil {
		t.Error("una voz inexistente debería fallar")
	}
}
