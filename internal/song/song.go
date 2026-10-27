// Package song define el modelo musical neutro que comparten el lector de
// Guitar Pro, el sintetizador, el exportador MIDI y el editor web.
package song

import "sort"

// PPQ es la resolución temporal: ticks por negra.
const PPQ = 960

// DefaultBPM es el tempo que se asume cuando el archivo no declara ninguno.
const DefaultBPM = 120

// Note es una nota ya colocada en la línea de tiempo de reproducción.
type Note struct {
	Start    int `json:"start"`    // inicio en ticks
	Duration int `json:"duration"` // duración en ticks
	Pitch    int `json:"pitch"`    // altura MIDI (60 = Do central); en percusión, número de instrumento GM
	Velocity int `json:"velocity"` // intensidad MIDI 1-127
}

// Track es una pista con sus notas ordenadas por inicio.
type Track struct {
	Name       string `json:"name"`
	Percussion bool   `json:"percussion"`
	Notes      []Note `json:"notes"`
}

// Tempo marca un cambio de tempo a partir de un tick.
type Tempo struct {
	Tick int     `json:"tick"`
	BPM  float64 `json:"bpm"`
}

// Song es una canción con las repeticiones ya expandidas, lista para reproducirse.
type Song struct {
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Tempos []Tempo `json:"tempos"` // ordenados por tick; el primero siempre está en el tick 0
	Bars   []int   `json:"bars"`   // tick de inicio de cada compás, en orden de reproducción
	Length int     `json:"length"` // duración total en ticks
	Tracks []Track `json:"tracks"`
}

// Clock convierte posiciones en ticks a segundos. Precalcula el instante de
// cada cambio de tempo para resolver cada consulta con una búsqueda binaria,
// ya que se consulta una vez por nota.
type Clock struct {
	tempos []Tempo
	starts []float64 // segundo en que empieza cada tempo
}

// Clock prepara la conversión de ticks a segundos según el mapa de tempo.
func (s *Song) Clock() *Clock {
	c := &Clock{tempos: s.Tempos, starts: make([]float64, len(s.Tempos))}
	for i := 1; i < len(s.Tempos); i++ {
		prev := s.Tempos[i-1]
		c.starts[i] = c.starts[i-1] + ticksToSeconds(s.Tempos[i].Tick-prev.Tick, prev.BPM)
	}
	return c
}

// Seconds devuelve el instante en segundos del tick indicado.
func (c *Clock) Seconds(tick int) float64 {
	i := sort.Search(len(c.tempos), func(i int) bool { return c.tempos[i].Tick > tick }) - 1
	if i < 0 {
		return ticksToSeconds(tick, DefaultBPM)
	}
	return c.starts[i] + ticksToSeconds(tick-c.tempos[i].Tick, c.tempos[i].BPM)
}

// Duration devuelve la duración total de la canción en segundos.
func (s *Song) Duration() float64 {
	return s.Clock().Seconds(s.Length)
}

func ticksToSeconds(ticks int, bpm float64) float64 {
	return float64(ticks) * 60 / (bpm * PPQ)
}
