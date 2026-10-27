package chip

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

const (
	// SampleRate es la frecuencia de muestreo del audio generado.
	SampleRate = 44100
	// MaxSeconds limita la duración renderizable para acotar el uso de memoria.
	MaxSeconds = 15 * 60
	// tail deja sonar las colas de release al final de la canción.
	tail = 1.0
)

// ErrTooLong indica que la canción supera MaxSeconds.
var ErrTooLong = fmt.Errorf("la canción supera el máximo de %d minutos", MaxSeconds/60)

// DefaultMapping propone una asignación de pistas a voces: la primera pista
// de percusión va al canal de ruido; la melódica más grave, a la última voz
// tonal (la de bajo en todos los presets), y el resto de melódicas, por
// orden de aparición, a las voces tonales restantes.
func DefaultMapping(s *song.Song, p Preset) []int {
	mapping := slices.Repeat([]int{-1}, len(p.Voices))
	var tonal, melodic []int
	drums := -1
	for i, v := range p.Voices {
		if !v.Drums() {
			tonal = append(tonal, i)
		}
	}
	avg := make([]float64, len(s.Tracks))
	for i, t := range s.Tracks {
		switch {
		case len(t.Notes) == 0:
		case t.Percussion:
			if drums < 0 {
				drums = i
			}
		default:
			melodic = append(melodic, i)
			for _, n := range t.Notes {
				avg[i] += float64(n.Pitch) / float64(len(t.Notes))
			}
		}
	}
	if len(melodic) > 1 && len(tonal) > 1 {
		low := 0
		for i, t := range melodic {
			if avg[t] < avg[melodic[low]] {
				low = i
			}
		}
		mapping[tonal[len(tonal)-1]] = melodic[low]
		melodic = slices.Delete(melodic, low, low+1)
		tonal = tonal[:len(tonal)-1]
	}
	for i := range min(len(melodic), len(tonal)) {
		mapping[tonal[i]] = melodic[i]
	}
	for i, v := range p.Voices {
		if v.Drums() {
			mapping[i] = drums
		}
	}
	return mapping
}

// Render sintetiza las partes (una por voz del preset, con sus notas
// ordenadas por inicio) y devuelve audio mono en [-1, 1] a SampleRate.
func Render(s *song.Song, p Preset, parts []song.Track) ([]float32, error) {
	clock := s.Clock()
	seconds := clock.Seconds(s.Length)
	if seconds > MaxSeconds {
		return nil, ErrTooLong
	}
	out := make([]float32, int((seconds+tail)*SampleRate))
	for i, part := range parts {
		renderVoice(out, clock, p, p.Voices[i], part.Notes)
	}
	master(out, p.Bits)
	return out, nil
}

// timedNote es una nota con sus límites expresados en muestras.
type timedNote struct {
	start, end, pitch int
	velocity          float64
}

// renderVoice suma al buffer una voz monofónica, como el hardware original:
// las notas simultáneas se alternan en arpegio a la frecuencia de refresco
// del chip y cada ataque reinicia la envolvente.
func renderVoice(out []float32, clock *song.Clock, p Preset, v Voice, notes []song.Note) {
	timed := make([]timedNote, 0, len(notes))
	for _, n := range notes {
		t := timedNote{
			start:    toSample(clock.Seconds(n.Start)),
			end:      toSample(clock.Seconds(n.Start + n.Duration)),
			pitch:    n.Pitch,
			velocity: float64(n.Velocity) / 127,
		}
		if t.end > t.start {
			timed = append(timed, t)
		}
	}
	if len(timed) == 0 {
		return
	}
	var (
		osc     = oscillator{lfsr: 1}
		active  []timedNote // notas sonando, ordenadas por altura
		current timedNote   // nota seleccionada (se conserva durante el release)
		onset   int         // muestra del último ataque
		off     = -1        // muestra en que se soltó la última nota (-1 = sonando)
		next    int         // siguiente nota por activar
		frame   = SampleRate / p.FrameRate
		// Como en los drivers originales, la altura (arpegio y vibrato) se
		// recalcula una vez por fotograma o al cambiar de nota.
		freq      float64
		freqPitch = -1
		freqFrame = -1
	)
	for n := timed[0].start; n < len(out); n++ {
		attacked := false
		for ; next < len(timed) && timed[next].start <= n; next++ {
			i, _ := slices.BinarySearchFunc(active, timed[next].pitch, func(t timedNote, p int) int { return cmp.Compare(t.pitch, p) })
			active = slices.Insert(active, i, timed[next])
			attacked = true
		}
		active = slices.DeleteFunc(active, func(t timedNote) bool { return t.end <= n })
		switch {
		case attacked:
			onset, off = n, -1
		case len(active) == 0 && off < 0:
			off = n
		}
		elapsed := float64(n-onset) / SampleRate
		frameIndex := int(float64(n-onset) / frame)
		if len(active) > 0 {
			current = pick(v, active, frameIndex)
		}

		var level float64
		var metallic bool
		var d drum // solo se rellena en percusión
		if v.Drums() {
			d = drumFor(current.pitch)
			level = max(0, 1-elapsed/d.decay)
			freq, metallic = d.clock, d.metallic
		} else {
			if current.pitch != freqPitch || frameIndex != freqFrame {
				freq = pitchToFreq(float64(current.pitch) + v.vibrato(elapsed))
				freqPitch, freqFrame = current.pitch, frameIndex
			}
			released := -1.0
			if off >= 0 {
				released = float64(off-onset) / SampleRate
			}
			if p.VolumeSteps > 0 {
				// Los chips actualizan el volumen una vez por fotograma.
				elapsed = math.Floor(elapsed*p.FrameRate) / p.FrameRate
			}
			level = v.envelope(elapsed, released)
		}
		if p.VolumeSteps > 0 {
			steps := float64(p.VolumeSteps)
			level = math.Round(level*steps) / steps
		}
		if level <= 0 && len(active) == 0 {
			// Silencio: se salta directamente al siguiente ataque.
			if next == len(timed) {
				return
			}
			n = timed[next].start - 1
			continue
		}
		value := osc.sample(v, freq, metallic)
		if d.body > 0 {
			// Cuerpo tonal con barrido descendente, como el triángulo que
			// usan los bombos y toms de muchos juegos de NES.
			osc.tone += d.tone * (1 - 0.6*elapsed/d.decay) / SampleRate
			value = (1-d.body)*value + d.body*math.Sin(2*math.Pi*osc.tone)
		}
		out[n] += float32(value * level * current.velocity)
	}
}

// pick elige qué nota suena en este fotograma: en percusión, el golpe más
// reciente (a igualdad, el más grave, que suele ser el bombo); en el resto,
// la nota del arpegio correspondiente.
func pick(v Voice, active []timedNote, frame int) timedNote {
	if v.Drums() {
		latest := active[0]
		for _, t := range active[1:] {
			if t.start > latest.start {
				latest = t
			}
		}
		return latest
	}
	return active[frame%len(active)]
}

// envelope calcula el volumen ADSR a los t segundos del ataque; released es
// el instante en que se soltó la nota (negativo si sigue pulsada).
func (v Voice) envelope(t, released float64) float64 {
	held := t
	if released >= 0 {
		held = released
	}
	var level float64
	switch {
	case held < v.Attack:
		level = held / v.Attack
	case held < v.Attack+v.Decay:
		level = 1 - (1-v.Sustain)*(held-v.Attack)/v.Decay
	default:
		level = v.Sustain
	}
	if released >= 0 {
		if v.Release <= 0 {
			return 0
		}
		level *= max(0, 1-(t-released)/v.Release)
	}
	return level
}

// vibrato devuelve la desviación de altura en semitonos; entra con retraso
// para que las notas cortas suenen limpias, como en los drivers de la época.
func (v Voice) vibrato(t float64) float64 {
	const delay, rate = 0.25, 5.5
	if v.Vibrato == 0 || t < delay {
		return 0
	}
	return v.Vibrato * math.Sin(2*math.Pi*rate*(t-delay))
}

// oscillator conserva la fase entre muestras para evitar chasquidos y el
// registro del generador de ruido.
type oscillator struct {
	phase float64 // ciclos acumulados
	tone  float64 // fase del cuerpo tonal de la percusión
	lfsr  uint16
}

func (o *oscillator) sample(v Voice, freq float64, metallic bool) float64 {
	o.phase += freq / SampleRate
	if v.Wave == Noise {
		// LFSR de 15 bits de la NES: realimenta el bit 0 con el 1 (ruido
		// blanco) o con el 6 (secuencia corta de timbre metálico).
		tap := uint16(1)
		if metallic {
			tap = 6
		}
		for ; o.phase >= 1; o.phase-- {
			bit := (o.lfsr ^ o.lfsr>>tap) & 1
			o.lfsr = o.lfsr>>1 | bit<<14
		}
		return float64(o.lfsr&1)*2 - 1
	}
	f := o.phase - math.Floor(o.phase)
	switch v.Wave {
	case Pulse:
		if f < v.Duty {
			return 1
		}
		return -1
	case Triangle:
		step := int(f * 32) // 32 pasos: sube 0-15 y baja 15-0
		if step >= 16 {
			step = 31 - step
		}
		return float64(step)/7.5 - 1
	case Saw:
		return 2*f - 1
	case Table:
		return v.Table[int(f*float64(len(v.Table)))]
	case FM:
		return math.Sin(2*math.Pi*o.phase + v.Index*math.Sin(2*math.Pi*o.phase*v.Ratio))
	}
	return 0
}

// drum describe el ruido con que se imita un instrumento de percusión.
type drum struct {
	clock    float64 // reloj del LFSR en Hz: más alto, más brillante
	decay    float64 // segundos hasta el silencio
	metallic bool
	tone     float64 // frecuencia inicial del cuerpo tonal en Hz
	body     float64 // proporción del cuerpo tonal frente al ruido (0 = solo ruido)
}

// drumFor traduce un instrumento de percusión General MIDI a un sonido de
// ruido, con cuerpo tonal en los parches graves para que no suenen iguales.
func drumFor(pitch int) drum {
	switch pitch {
	case 35, 36: // bombo
		return drum{clock: 1500, decay: 0.2, tone: 150, body: 0.8}
	case 37, 38, 39, 40: // caja, aro y palmas
		return drum{clock: 11000, decay: 0.16, tone: 220, body: 0.3}
	case 42, 44: // charles cerrado y de pie
		return drum{clock: 60000, decay: 0.05}
	case 46: // charles abierto
		return drum{clock: 60000, decay: 0.25}
	case 49, 52, 55, 57: // platos crash, china y splash
		return drum{clock: 35000, decay: 0.7}
	case 51, 53, 59: // ride
		return drum{clock: 45000, decay: 0.35, metallic: true}
	case 41, 43, 45, 47, 48, 50: // toms: más agudos cuanto mayor es la nota
		return drum{clock: 4000, decay: 0.22, tone: 80 + float64(pitch-41)*20, body: 0.7}
	default: // percusión auxiliar
		return drum{clock: max(1000, 3000+float64(pitch-40)*300), decay: 0.12}
	}
}

// Preview sintetiza una nota suelta (una corchea a 120 BPM) con el timbre de
// una voz, para oírla al colocarla en el editor tal como sonará.
func Preview(p Preset, voice, pitch int) ([]float32, error) {
	if voice < 0 || voice >= len(p.Voices) || pitch < 0 || pitch > 127 {
		return nil, errors.New("voz o altura fuera de rango")
	}
	const length = song.PPQ / 2
	s := &song.Song{Tempos: []song.Tempo{{Tick: 0, BPM: song.DefaultBPM}}, Length: length}
	parts := make([]song.Track, len(p.Voices))
	parts[voice].Notes = []song.Note{{Start: 0, Duration: length, Pitch: pitch, Velocity: 100}}
	out, err := Render(s, p, parts)
	// Se recorta el silencio final que Render reserva para las colas.
	for len(out) > 0 && out[len(out)-1] == 0 {
		out = out[:len(out)-1]
	}
	return out, err
}

// master elimina la componente continua (las ondas de pulso asimétricas la
// generan), normaliza el pico y reduce la resolución a la del DAC del chip.
func master(out []float32, bits int) {
	var prevIn, prevOut, peak float64
	for i, x := range out {
		y := float64(x) - prevIn + 0.995*prevOut
		prevIn, prevOut = float64(x), y
		out[i] = float32(y)
		peak = max(peak, math.Abs(y))
	}
	if peak == 0 {
		return
	}
	gain, levels := 0.9/peak, float64(int(1)<<max(bits-1, 0))
	for i, x := range out {
		y := float64(x) * gain
		if bits > 0 {
			y = math.Round(y*levels) / levels
		}
		out[i] = float32(y)
	}
}

func toSample(seconds float64) int { return int(seconds * SampleRate) }

func pitchToFreq(pitch float64) float64 { return 440 * math.Pow(2, (pitch-69)/12) }
