package server

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/Marcusdeks/FamiGp/internal/chip"
	"github.com/Marcusdeks/FamiGp/internal/song"
)

// Límites de un proyecto editado: protegen al servidor de peticiones que
// agotarían la memoria o desbordarían los cálculos de tiempo.
const (
	maxTick   = 1 << 30
	maxNotes  = 1 << 20
	maxTempos = 1 << 16
	maxBPM    = 1000
)

// projectRequest es el proyecto tal como lo edita el cliente: el mapa de
// tempo de la canción original y una línea de notas por canal del chip.
type projectRequest struct {
	Title  string        `json:"title"`
	Preset string        `json:"preset"`
	Tempos []song.Tempo  `json:"tempos"`
	Length int           `json:"length"`
	Lines  []lineRequest `json:"lines"`
}

// lineRequest son las notas de un canal y su transposición en semitonos,
// que se aplica al sintetizar sin modificar las notas guardadas.
type lineRequest struct {
	Transpose int         `json:"transpose"`
	Notes     []song.Note `json:"notes"`
}

// readProject decodifica y valida el proyecto del cuerpo de la petición y
// devuelve la canción, el preset y una parte por voz lista para sintetizar.
func readProject(w http.ResponseWriter, r *http.Request) (*song.Song, chip.Preset, []song.Track, error) {
	var req projectRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxUpload)).Decode(&req); err != nil {
		return nil, chip.Preset{}, nil, badRequest(errors.New("proyecto ilegible o demasiado grande"))
	}
	p, ok := chip.Find(req.Preset)
	if !ok {
		return nil, chip.Preset{}, nil, badRequest(errors.New("chip desconocido"))
	}
	if len(req.Lines) != len(p.Voices) {
		return nil, chip.Preset{}, nil, badRequest(errors.New("el número de canales no coincide con el chip"))
	}
	if err := validateTempos(req.Tempos); err != nil {
		return nil, chip.Preset{}, nil, badRequest(err)
	}
	if req.Length < 0 || req.Length > maxTick {
		return nil, chip.Preset{}, nil, badRequest(errors.New("duración del proyecto inválida"))
	}

	s := &song.Song{Title: req.Title, Tempos: req.Tempos, Length: req.Length}
	parts := make([]song.Track, len(p.Voices))
	total := 0
	for i, v := range p.Voices {
		line := req.Lines[i]
		if total += len(line.Notes); total > maxNotes {
			return nil, chip.Preset{}, nil, badRequest(errors.New("el proyecto tiene demasiadas notas"))
		}
		notes := make([]song.Note, len(line.Notes))
		for j, n := range line.Notes {
			n.Pitch += line.Transpose
			if err := validateNote(n); err != nil {
				return nil, chip.Preset{}, nil, badRequest(fmt.Errorf("%s: %w", v.Name, err))
			}
			notes[j] = n
			s.Length = max(s.Length, n.Start+n.Duration)
		}
		// El editor guarda las notas en el orden en que se crearon; el
		// sintetizador las necesita ordenadas por inicio.
		slices.SortStableFunc(notes, func(a, b song.Note) int { return cmp.Compare(a.Start, b.Start) })
		parts[i] = song.Track{Name: v.Name, Percussion: v.Drums(), Notes: notes}
	}
	return s, p, parts, nil
}

// validateTempos exige un mapa de tempo que empiece en el tick 0, avance en
// orden y use tempos razonables.
func validateTempos(tempos []song.Tempo) error {
	if len(tempos) == 0 || len(tempos) > maxTempos || tempos[0].Tick != 0 {
		return errors.New("mapa de tempo inválido")
	}
	for i, t := range tempos {
		if t.BPM < 1 || t.BPM > maxBPM || t.Tick > maxTick || i > 0 && t.Tick < tempos[i-1].Tick {
			return errors.New("mapa de tempo inválido")
		}
	}
	return nil
}

func validateNote(n song.Note) error {
	switch {
	case n.Start < 0 || n.Duration <= 0 || n.Start > maxTick || n.Duration > maxTick:
		return errors.New("nota con posición o duración inválida")
	case n.Pitch < 0 || n.Pitch > 127:
		return errors.New("la transposición deja notas fuera del rango MIDI (0-127)")
	case n.Velocity < 1 || n.Velocity > 127:
		return errors.New("nota con intensidad inválida")
	}
	return nil
}
