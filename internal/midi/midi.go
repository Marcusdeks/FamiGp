// Package midi escribe archivos MIDI estándar (SMF formato 1), el formato
// que FamiStudio importa para convertir una canción a canales de la NES.
package midi

import (
	"cmp"
	"encoding/binary"
	"io"
	"math"
	"slices"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

const drumChannel = 9 // canal 10 de General MIDI, reservado a percusión

// event es un mensaje MIDI con su posición absoluta en ticks.
type event struct {
	tick int
	data []byte
}

// Write genera un MIDI con una pista de tempo seguida de una pista por cada
// track (en el orden recibido), de modo que cada canal del chip llegue a
// FamiStudio como una pista independiente.
func Write(w io.Writer, s *song.Song, tracks []song.Track) error {
	be := binary.BigEndian
	header := be.AppendUint16(nil, 1) // formato 1: pistas simultáneas
	header = be.AppendUint16(header, uint16(len(tracks)+1))
	header = be.AppendUint16(header, song.PPQ)
	out := chunk(nil, "MThd", header)

	conductor := []event{meta(0, 0x03, s.Title)}
	for _, t := range s.Tempos {
		usPerQuarter := uint32(math.Round(60e6 / t.BPM))
		conductor = append(conductor, event{t.Tick, []byte{0xFF, 0x51, 3, byte(usPerQuarter >> 16), byte(usPerQuarter >> 8), byte(usPerQuarter)}})
	}
	out = chunk(out, "MTrk", encode(conductor))

	for i, t := range tracks {
		channel := byte(i)
		if t.Percussion {
			channel = drumChannel
		}
		events := []event{meta(0, 0x03, t.Name)}
		for _, n := range t.Notes {
			pitch, velocity := byte(min(max(n.Pitch, 0), 127)), byte(min(max(n.Velocity, 1), 127))
			events = append(events,
				event{n.Start, []byte{0x90 | channel, pitch, velocity}},
				event{n.Start + n.Duration, []byte{0x80 | channel, pitch, 0}},
			)
		}
		out = chunk(out, "MTrk", encode(events))
	}
	_, err := w.Write(out)
	return err
}

// meta crea un metaevento de texto (0x03 = nombre de pista).
func meta(tick int, kind byte, text string) event {
	return event{tick, append(appendVarLen([]byte{0xFF, kind}, uint32(len(text))), text...)}
}

// encode ordena los eventos y los serializa con tiempos delta. A igual tick,
// los note-off van antes que los note-on para que una nota repetida no se
// corte a sí misma.
func encode(events []event) []byte {
	slices.SortStableFunc(events, func(a, b event) int {
		return cmp.Or(cmp.Compare(a.tick, b.tick), cmp.Compare(isNoteOn(a), isNoteOn(b)))
	})
	var b []byte
	last := 0
	for _, e := range events {
		b = appendVarLen(b, uint32(e.tick-last))
		b = append(b, e.data...)
		last = e.tick
	}
	return append(b, 0x00, 0xFF, 0x2F, 0x00) // fin de pista
}

func isNoteOn(e event) int {
	if e.data[0]&0xF0 == 0x90 {
		return 1
	}
	return 0
}

// appendVarLen codifica un entero en el formato de longitud variable de MIDI:
// 7 bits por byte, con el bit alto activo en todos menos el último.
func appendVarLen(b []byte, v uint32) []byte {
	var buf [5]byte
	i := len(buf) - 1
	buf[i] = byte(v & 0x7F)
	for v >>= 7; v > 0; v >>= 7 {
		i--
		buf[i] = byte(v&0x7F) | 0x80
	}
	return append(b, buf[i:]...)
}

// chunk añade un bloque MIDI: identificador, longitud de 32 bits y contenido.
func chunk(b []byte, id string, body []byte) []byte {
	b = append(b, id...)
	b = binary.BigEndian.AppendUint32(b, uint32(len(body)))
	return append(b, body...)
}
