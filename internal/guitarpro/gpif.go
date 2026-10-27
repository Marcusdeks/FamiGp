package guitarpro

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

// parseGP7 abre un archivo de Guitar Pro 7/8: un ZIP que contiene la
// partitura en XML (formato GPIF) en Content/score.gpif.
func parseGP7(data []byte) (*score, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, ErrCorrupt
	}
	f, err := zr.Open("Content/score.gpif")
	if err != nil {
		return nil, ErrCorrupt
	}
	defer f.Close()
	// El límite protege de bombas ZIP (archivos diminutos que se descomprimen a gigabytes).
	xmlData, err := io.ReadAll(io.LimitReader(f, maxXMLSize+1))
	if err != nil || len(xmlData) > maxXMLSize {
		return nil, ErrCorrupt
	}
	return parseGPIF(xmlData)
}

// Estructuras XML de GPIF. Solo se declaran los campos que se usan; el
// decodificador ignora el resto. Las entidades se referencian entre sí por id.
type gpif struct {
	Title       string `xml:"Score>Title"`
	Artist      string `xml:"Score>Artist"`
	Automations []struct {
		Type     string
		Bar      int
		Position float64 // fracción del compás (0-1)
		Value    string  // "120 2": tempo y figura de referencia
	} `xml:"MasterTrack>Automations>Automation"`
	Tracks     []gpTrack     `xml:"Tracks>Track"`
	MasterBars []gpMasterBar `xml:"MasterBars>MasterBar"`
	Bars       []gpBar       `xml:"Bars>Bar"`
	Voices     []gpVoice     `xml:"Voices>Voice"`
	Beats      []gpBeat      `xml:"Beats>Beat"`
	Notes      []gpNote      `xml:"Notes>Note"`
	Rhythms    []gpRhythm    `xml:"Rhythms>Rhythm"`
}

type gpTrack struct {
	Name          string
	Properties    []gpProperty `xml:"Properties>Property"`              // GP6
	Staves        []gpProperty `xml:"Staves>Staff>Properties>Property"` // GP7+
	Channel       *int         `xml:"MidiConnection>PrimaryChannel"`
	KitType       string       `xml:"InstrumentSet>Type"`
	Articulations []int        `xml:"InstrumentSet>Elements>Element>Articulations>Articulation>OutputMidiNumber"`
}

type gpMasterBar struct {
	Time   string
	Repeat struct {
		Start bool `xml:"start,attr"`
		End   bool `xml:"end,attr"`
		Count int  `xml:"count,attr"`
	}
	AlternateEndings intList
	Bars             intList // id de compás por pista
}

// node aporta el atributo id común y permite indexar cualquier entidad.
type node struct {
	ID int `xml:"id,attr"`
}

func (n node) key() int { return n.ID }

type gpBar struct {
	node
	Voices intList
}

type gpVoice struct {
	node
	Beats intList
}

type gpBeat struct {
	node
	Rhythm struct {
		Ref int `xml:"ref,attr"`
	}
	Notes      intList
	Dynamic    string
	GraceNotes string
}

type gpNote struct {
	node
	Tie struct {
		Destination bool `xml:"destination,attr"`
	}
	Articulation *int         `xml:"InstrumentArticulation"`
	Properties   []gpProperty `xml:"Properties>Property"`
}

type gpRhythm struct {
	node
	NoteValue string
	Dot       struct {
		Count int `xml:"count,attr"`
	} `xml:"AugmentationDot"`
	Tuplet struct {
		Num int `xml:"num,attr"`
		Den int `xml:"den,attr"`
	} `xml:"PrimaryTuplet"`
}

// gpProperty es la propiedad genérica <Property name="..."> de GPIF; cada
// nombre rellena un campo distinto.
type gpProperty struct {
	Name    string  `xml:"name,attr"`
	Pitches intList // Tuning
	Fret    *int    // Fret, CapoFret
	String  *int
	Number  *int      // Midi
	Enable  *struct{} // Muted
}

// index construye un mapa id → entidad.
func index[T interface{ key() int }](items []T) map[int]T {
	m := make(map[int]T, len(items))
	for _, it := range items {
		m[it.key()] = it
	}
	return m
}

// intList es una lista de enteros separados por espacios ("0 1 -1"), la
// forma en que GPIF enlaza entidades. Se decodifica una sola vez al leer el
// XML; los valores no numéricos se tratan como -1 (referencia vacía).
type intList []int

func (l *intList) UnmarshalText(text []byte) error {
	fields := strings.Fields(string(text))
	*l = make(intList, len(fields))
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			n = -1
		}
		(*l)[i] = n
	}
	return nil
}

var noteValues = map[string]int{
	"Whole": 4 * song.PPQ, "Half": 2 * song.PPQ, "Quarter": song.PPQ, "Eighth": song.PPQ / 2,
	"16th": song.PPQ / 4, "32nd": song.PPQ / 8, "64th": song.PPQ / 16, "128th": song.PPQ / 32,
}

// ticks calcula la duración de una figura con sus puntillos y su grupo irregular.
func (r gpRhythm) ticks() int {
	d := noteValues[r.NoteValue]
	for add := d; r.Dot.Count > 0; r.Dot.Count-- {
		add /= 2
		d += add
	}
	if r.Tuplet.Num > 0 && r.Tuplet.Den > 0 {
		d = d * r.Tuplet.Den / r.Tuplet.Num
	}
	return d
}

// gpifTrack reúne lo necesario para calcular la altura de las notas de una pista.
type gpifTrack struct {
	tuning        []int // de la cuerda más grave a la más aguda, cejilla incluida
	articulations []int // número MIDI de salida de cada articulación del kit de batería
	percussion    bool
}

// parseGPIF interpreta el XML de partitura común a Guitar Pro 6, 7 y 8.
func parseGPIF(data []byte) (*score, error) {
	var doc gpif
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, ErrCorrupt
	}
	if len(doc.MasterBars) == 0 || len(doc.MasterBars) > maxBars || len(doc.Tracks) > maxTracks {
		return nil, ErrCorrupt
	}
	sc := &score{title: doc.Title, artist: doc.Artist, tempo: song.DefaultBPM}
	for _, mb := range doc.MasterBars {
		var b masterBar
		if _, err := fmt.Sscanf(mb.Time, "%d/%d", &b.num, &b.den); err != nil || !b.valid() {
			return nil, ErrCorrupt
		}
		b.repeatStart = mb.Repeat.Start
		if mb.Repeat.End {
			b.repeatPlays = max(mb.Repeat.Count, 2)
		}
		for _, n := range mb.AlternateEndings {
			if n >= 1 && n <= 8 {
				b.alternates |= 1 << (n - 1)
			}
		}
		sc.bars = append(sc.bars, b)
	}
	for _, a := range doc.Automations {
		bpm, err := strconv.ParseFloat(strings.Fields(a.Value + " 0")[0], 64)
		if a.Type != "Tempo" || err != nil || bpm <= 0 || a.Bar < 0 || a.Bar >= len(sc.bars) {
			continue
		}
		if a.Bar == 0 && a.Position == 0 {
			sc.tempo = bpm
			continue
		}
		b := &sc.bars[a.Bar]
		b.tempos = append(b.tempos, tempoMark{int(a.Position * float64(b.length())), bpm})
	}

	infos := make([]gpifTrack, len(doc.Tracks))
	for i, t := range doc.Tracks {
		info := &infos[i]
		capo := 0
		for _, p := range slices.Concat(t.Properties, t.Staves) {
			switch {
			case p.Name == "Tuning" && info.tuning == nil:
				info.tuning = p.Pitches
			case p.Name == "CapoFret" && p.Fret != nil:
				capo = *p.Fret
			}
		}
		for s := range info.tuning {
			info.tuning[s] += capo
		}
		info.articulations = t.Articulations
		info.percussion = t.KitType == "drumKit" || t.Channel != nil && *t.Channel == 9
		sc.tracks = append(sc.tracks, newTrack(t.Name, info.percussion, len(sc.bars)))
	}

	bars, voices, beats := index(doc.Bars), index(doc.Voices), index(doc.Beats)
	notes, rhythms := index(doc.Notes), index(doc.Rhythms)
	starts := sc.barStarts()
	events := 0 // las referencias compartidas permiten reutilizar un pulso en muchos compases
	for bi, mb := range doc.MasterBars {
		for ti, barID := range mb.Bars {
			if ti >= len(sc.tracks) {
				break
			}
			for slot, voiceID := range bars[barID].Voices {
				if voiceID < 0 {
					continue
				}
				offset := 0
				for _, beatID := range voices[voiceID].Beats {
					if events++; events > maxEvents {
						return nil, ErrCorrupt
					}
					beat := beats[beatID]
					// Las notas de adorno no consumen tiempo del compás.
					if beat.GraceNotes != "" {
						continue
					}
					duration := rhythms[beat.Rhythm.Ref].ticks()
					velocity, ok := velocities[beat.Dynamic]
					if !ok {
						velocity = defaultVelocity
					}
					for _, noteID := range beat.Notes {
						n := notes[noteID]
						if pitch, key, ok := infos[ti].pitch(n); ok {
							sc.tracks[ti].add(starts, bi, slot<<8|key, note{offset, duration, pitch, velocity}, n.Tie.Destination)
						}
					}
					offset += duration
				}
			}
		}
	}
	return sc, nil
}

// pitch obtiene la altura MIDI de una nota y la clave con la que se enlazan
// sus ligaduras. En percusión manda la articulación del kit; en instrumentos
// de cuerda, cuerda y traste (la clave es la cuerda, porque un bend cambia la
// altura escrita de la continuación); como último recurso, el número MIDI
// explícito. Las notas apagadas no suenan.
func (t gpifTrack) pitch(n gpNote) (pitch, key int, ok bool) {
	var str, fret, midi *int
	for _, p := range n.Properties {
		switch p.Name {
		case "String":
			str = p.String
		case "Fret":
			fret = p.Fret
		case "Midi":
			midi = p.Number
		case "Muted":
			if p.Enable != nil {
				return 0, 0, false
			}
		}
	}
	switch {
	case t.percussion && n.Articulation != nil && *n.Articulation >= 0 && *n.Articulation < len(t.articulations):
		pitch = t.articulations[*n.Articulation]
		return pitch, pitch, true
	case str != nil && fret != nil && *str >= 0 && *str < len(t.tuning):
		return t.tuning[*str] + *fret, 128 + *str, true // claves 128+ reservadas a cuerdas
	case midi != nil:
		return *midi, *midi, true
	}
	return 0, 0, false
}
