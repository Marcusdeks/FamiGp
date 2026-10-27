package song

import (
	"math"
	"testing"
)

func TestClock(t *testing.T) {
	// Dos negras a 120 BPM (0,5 s cada una) y después a 60 BPM (1 s cada una).
	s := &Song{Tempos: []Tempo{{0, 120}, {2 * PPQ, 60}}, Length: 4 * PPQ}
	clock := s.Clock()
	for tick, want := range map[int]float64{0: 0, PPQ: 0.5, 2 * PPQ: 1, 3 * PPQ: 2, 4 * PPQ: 3} {
		if got := clock.Seconds(tick); math.Abs(got-want) > 1e-9 {
			t.Errorf("Seconds(%d) = %v, se esperaba %v", tick, got, want)
		}
	}
	if got := s.Duration(); got != 3 {
		t.Errorf("Duration() = %v, se esperaba 3", got)
	}
}

func TestClockWithoutTempo(t *testing.T) {
	if got := (&Song{}).Clock().Seconds(PPQ); got != 0.5 {
		t.Errorf("sin mapa de tempo, una negra = %v s; se esperaba 0,5 s (%d BPM)", got, DefaultBPM)
	}
}
