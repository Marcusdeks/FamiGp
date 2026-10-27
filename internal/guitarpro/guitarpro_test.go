package guitarpro

import (
	"archive/zip"
	"bytes"
	"errors"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

// gp7File empaqueta testdata/score.gpif como un archivo .gp de Guitar Pro 7.
func gp7File(t testing.TB) []byte {
	t.Helper()
	xmlData, err := os.ReadFile("testdata/score.gpif")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("Content/score.gpif")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(xmlData); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseGP7(t *testing.T) {
	s, err := Parse(gp7File(t))
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "Prueba" || s.Artist != "FamiGP" {
		t.Errorf("metadatos = %q / %q", s.Title, s.Artist)
	}
	if !slices.Equal(s.Bars, []int{0, 3840}) || s.Length != 7680 {
		t.Errorf("compases = %v, duración = %d ticks", s.Bars, s.Length)
	}
	if got := s.Duration(); math.Abs(got-16.0/3) > 1e-9 {
		t.Errorf("duración = %v s, se esperaban 8 negras a 90 BPM", got)
	}
	// La nota ligada se funde con su origen: 3er traste de la cuerda Mi aguda
	// (64 + 3) durante dos compases, con la dinámica mf del primer pulso.
	guitar := song.Track{Name: "Guitarra", Notes: []song.Note{{Start: 0, Duration: 2 * 3840, Pitch: 67, Velocity: 79}}}
	if !equalTrack(s.Tracks[0], guitar) {
		t.Errorf("guitarra = %+v, se esperaba %+v", s.Tracks[0], guitar)
	}
	drums := song.Track{Name: "Batería", Percussion: true, Notes: []song.Note{
		{Start: 0, Duration: 1920, Pitch: 36, Velocity: defaultVelocity},
		{Start: 1920, Duration: 1920, Pitch: 38, Velocity: defaultVelocity},
		{Start: 3840, Duration: 1920, Pitch: 36, Velocity: defaultVelocity},
		{Start: 5760, Duration: 1920, Pitch: 38, Velocity: defaultVelocity},
	}}
	if !equalTrack(s.Tracks[1], drums) {
		t.Errorf("batería = %+v, se esperaba %+v", s.Tracks[1], drums)
	}
}

func equalTrack(a, b song.Track) bool {
	return a.Name == b.Name && a.Percussion == b.Percussion && slices.Equal(a.Notes, b.Notes)
}

func TestParseErrors(t *testing.T) {
	var empty bytes.Buffer
	if err := zip.NewWriter(&empty).Close(); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"desconocido", []byte("no es una partitura"), ErrUnsupported},
		{"vacío", nil, ErrUnsupported},
		{"GP5 truncado", append([]byte{24}, "FICHIER GUITAR PRO v5.00"...), ErrCorrupt},
		{"ZIP sin partitura", empty.Bytes(), ErrCorrupt},
		{"GPX truncado", []byte("BCFZ\x10\x00"), ErrCorrupt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.data); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, se esperaba %v", err, tt.want)
			}
		})
	}
}

func TestPlayOrder(t *testing.T) {
	tests := []struct {
		name string
		bars []masterBar
		want []int
	}{
		{"sin repeticiones", []masterBar{{}, {}, {}}, []int{0, 1, 2}},
		{"repetición sin inicio explícito", []masterBar{{}, {repeatPlays: 2}, {}}, []int{0, 1, 0, 1, 2}},
		{
			"secciones consecutivas",
			[]masterBar{{repeatStart: true}, {repeatPlays: 2}, {repeatStart: true}, {repeatPlays: 3}},
			[]int{0, 1, 0, 1, 2, 3, 2, 3, 2, 3},
		},
		{
			"finales alternativos 1-3 y 2 con cuatro pasadas",
			[]masterBar{{repeatStart: true}, {alternates: 0b101}, {alternates: 0b010}, {repeatPlays: 4}, {}},
			[]int{0, 1, 3, 0, 2, 3, 0, 1, 3, 0, 3, 4},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (&score{bars: tt.bars}).playOrder(); !slices.Equal(got, tt.want) {
				t.Errorf("orden = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}

func TestRhythmTicks(t *testing.T) {
	tests := []struct {
		rhythm gpRhythm
		want   int
	}{
		{gpRhythm{NoteValue: "Quarter"}, 960},
		{gpRhythm{NoteValue: "Quarter", Dot: struct {
			Count int `xml:"count,attr"`
		}{2}}, 960 + 480 + 240},
		{gpRhythm{NoteValue: "Eighth", Tuplet: struct {
			Num int `xml:"num,attr"`
			Den int `xml:"den,attr"`
		}{3, 2}}, 320},
	}
	for _, tt := range tests {
		if got := tt.rhythm.ticks(); got != tt.want {
			t.Errorf("%+v = %d ticks, se esperaban %d", tt.rhythm, got, tt.want)
		}
	}
}

func TestTuplet(t *testing.T) {
	for n, want := range map[int]int{1: 960, 3: 640, 5: 768, 6: 640, 7: 548, 9: 853} {
		if got := tuplet(960, n); got != want {
			t.Errorf("tuplet(960, %d) = %d, se esperaba %d", n, got, want)
		}
	}
}

// bitString empaqueta una cadena de ceros y unos en bytes, de mayor a menor peso.
func bitString(s string) []byte {
	s = strings.ReplaceAll(s, " ", "")
	out := make([]byte, (len(s)+7)/8)
	for i, c := range s {
		if c == '1' {
			out[i/8] |= 0x80 >> (i % 8)
		}
	}
	return out
}

func TestDecompressBCFZ(t *testing.T) {
	// Literal de 3 bytes ("abc") y una referencia de distancia 3 y tamaño 3.
	// Los campos de tamaño se leen con el bit menos significativo primero.
	stream := bitString("0 11 01100001 01100010 01100011" + "1 0010 11 11")
	data := append([]byte{6, 0, 0, 0}, stream...)
	got, err := decompressBCFZ(data)
	if err != nil || string(got) != "abcabc" {
		t.Errorf("decompressBCFZ = %q, %v; se esperaba \"abcabc\"", got, err)
	}
	if _, err := decompressBCFZ(append([]byte{6, 0, 0, 0}, bitString("1 0010 11 11")...)); err == nil {
		t.Error("una referencia a datos inexistentes debería fallar")
	}
}

// FuzzParse comprueba que ninguna entrada, por dañada que esté, provoca un
// pánico o una canción nula sin error.
func FuzzParse(f *testing.F) {
	f.Add(gp7File(f))
	f.Add(append([]byte{24}, "FICHIER GUITAR PRO v5.00"...))
	f.Add([]byte("BCFZ\x10\x00\x00\x00"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if s, err := Parse(data); err == nil && s == nil {
			t.Fatal("Parse devolvió una canción nula sin error")
		}
	})
}
