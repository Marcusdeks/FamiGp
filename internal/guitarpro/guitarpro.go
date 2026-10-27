// Package guitarpro lee partituras de Guitar Pro 3, 4, 5 (.gp3/.gp4/.gp5),
// 6 (.gpx) y 7/8 (.gp) y las convierte al modelo de reproducción de song.
package guitarpro

import (
	"bytes"
	"cmp"
	"errors"
	"slices"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

// Límites de cordura frente a archivos dañados o maliciosos: evitan reservas
// de memoria gigantes y la amplificación que permiten las repeticiones y las
// referencias compartidas de GPIF. Quedan muy por encima de cualquier
// partitura real.
const (
	maxBars       = 4096     // compases escritos
	maxTracks     = 128      // pistas
	maxEvents     = 1 << 20  // pulsos procesados al leer
	maxPlayedBars = 16384    // compases tras expandir repeticiones
	maxNotes      = 1 << 21  // notas tras expandir repeticiones
	maxXMLSize    = 64 << 20 // XML de partitura descomprimido
)

var (
	// ErrUnsupported indica que los datos no son de ningún formato de Guitar Pro reconocido.
	ErrUnsupported = errors.New("formato de Guitar Pro no reconocido")
	// ErrCorrupt indica que el archivo tiene un formato reconocido pero su contenido es inválido.
	ErrCorrupt = errors.New("archivo de Guitar Pro dañado o incompleto")
)

// Parse detecta el formato por su firma y devuelve la canción con las
// repeticiones y los finales alternativos ya expandidos.
func Parse(data []byte) (*song.Song, error) {
	var (
		sc  *score
		err error
	)
	switch {
	case bytes.HasPrefix(data, []byte("PK")):
		sc, err = parseGP7(data)
	case bytes.HasPrefix(data, []byte("BCF")):
		sc, err = parseGPX(data)
	case len(data) > 1 && bytes.HasPrefix(data[1:], []byte("FICHIER GUITAR PRO")):
		sc, err = parseBinary(data)
	default:
		return nil, ErrUnsupported
	}
	if err != nil {
		return nil, err
	}
	return sc.unroll(), nil
}

// score es la representación intermedia común a todos los formatos: la
// partitura tal como está escrita, antes de desplegar las repeticiones.
type score struct {
	title, artist string
	tempo         float64
	bars          []masterBar
	tracks        []*track
}

// masterBar contiene los datos de un compás compartidos por todas las pistas.
type masterBar struct {
	num, den    int         // compás, p. ej. 3/4
	repeatStart bool        // abre una sección de repetición
	repeatPlays int         // veces que se toca la sección al cerrarla (0 = no cierra)
	alternates  int         // máscara de finales alternativos (bit 0 = primera vuelta)
	tempos      []tempoMark // cambios de tempo dentro del compás
}

type tempoMark struct {
	offset int // ticks desde el inicio del compás
	bpm    float64
}

// length devuelve la duración del compás en ticks según su indicación de compás.
func (b masterBar) length() int {
	return b.num * 4 * song.PPQ / b.den
}

// valid descarta indicaciones de compás imposibles que provocarían divisiones por cero.
func (b masterBar) valid() bool {
	return b.num > 0 && b.den > 0
}

// track guarda las notas de una pista agrupadas por compás escrito.
type track struct {
	name       string
	percussion bool
	bars       [][]note
	open       map[int]noteRef // última nota de cada cuerda/voz, para resolver ligaduras
}

// note es una nota con su posición relativa al inicio de su compás.
type note struct {
	offset, duration, pitch, velocity int
}

type noteRef struct{ bar, index int }

func newTrack(name string, percussion bool, bars int) *track {
	return &track{name: name, percussion: percussion, bars: make([][]note, bars), open: map[int]noteRef{}}
}

// add registra una nota en el compás bar. Si es la continuación de una
// ligadura, en lugar de añadirla alarga la nota de origen (identificada por
// key) hasta el final de la continuación; una ligadura sin origen suena como
// nota normal. starts son los inicios absolutos de cada compás escrito,
// necesarios porque la ligadura puede cruzar compases.
func (t *track) add(starts []int, bar, key int, n note, tied bool) {
	if ref, ok := t.open[key]; ok && tied {
		origin := &t.bars[ref.bar][ref.index]
		origin.duration = starts[bar] + n.offset + n.duration - starts[ref.bar] - origin.offset
		return
	}
	t.bars[bar] = append(t.bars[bar], n)
	t.open[key] = noteRef{bar, len(t.bars[bar]) - 1}
}

// barStarts devuelve el tick absoluto de inicio de cada compás en orden escrito.
func (sc *score) barStarts() []int {
	starts := make([]int, len(sc.bars))
	for i := 1; i < len(sc.bars); i++ {
		starts[i] = starts[i-1] + sc.bars[i-1].length()
	}
	return starts
}

// playOrder devuelve los índices de compás en el orden en que suenan,
// expandiendo repeticiones y saltando los finales alternativos que no
// corresponden a la vuelta actual.
func (sc *score) playOrder() []int {
	order := make([]int, 0, len(sc.bars))
	start, pass := 0, 1
	for i := 0; i < len(sc.bars) && len(order) < maxPlayedBars; {
		b := sc.bars[i]
		if b.repeatStart && i != start {
			start, pass = i, 1
		}
		if b.alternates != 0 && b.alternates&(1<<(pass-1)) == 0 {
			i++
			continue
		}
		order = append(order, i)
		switch {
		case b.repeatPlays > pass:
			pass++
			i = start
		case b.repeatPlays > 0:
			// Fin de la repetición: si no hay inicio explícito, la siguiente
			// sección repetida empieza justo después de este compás.
			start, pass = i+1, 1
			i++
		default:
			i++
		}
	}
	return order
}

// unroll coloca cada compás en la línea de tiempo según el orden de
// reproducción y genera la canción final.
func (sc *score) unroll() *song.Song {
	s := &song.Song{
		Title:  sc.title,
		Artist: sc.artist,
		Tempos: []song.Tempo{{Tick: 0, BPM: sc.tempo}},
		Tracks: make([]song.Track, len(sc.tracks)),
	}
	for i, t := range sc.tracks {
		s.Tracks[i] = song.Track{Name: t.name, Percussion: t.percussion}
	}
	tick, notes := 0, 0
	for _, i := range sc.playOrder() {
		if notes > maxNotes {
			break
		}
		b := sc.bars[i]
		s.Bars = append(s.Bars, tick)
		for _, m := range b.tempos {
			s.Tempos = append(s.Tempos, song.Tempo{Tick: tick + m.offset, BPM: m.bpm})
		}
		for ti, t := range sc.tracks {
			notes += len(t.bars[i])
			for _, n := range t.bars[i] {
				s.Tracks[ti].Notes = append(s.Tracks[ti].Notes, song.Note{
					Start: tick + n.offset, Duration: n.duration, Pitch: n.pitch, Velocity: n.velocity,
				})
			}
		}
		tick += b.length()
	}
	s.Length = tick
	// Las voces de un mismo compás se leen por separado, así que hay que
	// reordenar por tiempo; el orden estable conserva el de los acordes.
	slices.SortStableFunc(s.Tempos, func(a, b song.Tempo) int { return cmp.Compare(a.Tick, b.Tick) })
	for i := range s.Tracks {
		slices.SortStableFunc(s.Tracks[i].Notes, func(a, b song.Note) int { return cmp.Compare(a.Start, b.Start) })
	}
	return s
}

// velocities traduce las dinámicas de Guitar Pro (ppp..fff) a intensidad MIDI.
var velocities = map[string]int{"PPP": 15, "PP": 31, "P": 47, "MP": 63, "MF": 79, "F": 95, "FF": 111, "FFF": 127}

// defaultVelocity corresponde a forte, la dinámica por defecto de Guitar Pro.
const defaultVelocity = 95
