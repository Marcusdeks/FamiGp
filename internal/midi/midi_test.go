package midi

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

func TestAppendVarLen(t *testing.T) {
	tests := []struct {
		v    uint32
		want []byte
	}{
		{0, []byte{0x00}},
		{0x7F, []byte{0x7F}},
		{0x80, []byte{0x81, 0x00}},
		{0x3FFF, []byte{0xFF, 0x7F}},
		{0x200000, []byte{0x81, 0x80, 0x80, 0x00}},
	}
	for _, tt := range tests {
		if got := appendVarLen(nil, tt.v); !bytes.Equal(got, tt.want) {
			t.Errorf("appendVarLen(%#x) = % x, se esperaba % x", tt.v, got, tt.want)
		}
	}
}

func TestWrite(t *testing.T) {
	s := &song.Song{Title: "Demo", Tempos: []song.Tempo{{Tick: 0, BPM: 120}}}
	tracks := []song.Track{
		// Dos notas iguales consecutivas: el note-off de la primera debe ir antes que el note-on de la segunda.
		{Name: "Pulso 1", Notes: []song.Note{{Start: 0, Duration: 480, Pitch: 60, Velocity: 100}, {Start: 480, Duration: 480, Pitch: 60, Velocity: 100}}},
		{Name: "Ruido", Percussion: true, Notes: []song.Note{{Start: 0, Duration: 240, Pitch: 36, Velocity: 90}}},
	}
	var buf bytes.Buffer
	if err := Write(&buf, s, tracks); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if string(b[:4]) != "MThd" || binary.BigEndian.Uint16(b[10:]) != 3 || binary.BigEndian.Uint16(b[12:]) != song.PPQ {
		t.Fatalf("cabecera inválida: % x", b[:14])
	}
	// 500 000 µs por negra = 120 BPM.
	if !bytes.Contains(b, []byte{0xFF, 0x51, 0x03, 0x07, 0xA1, 0x20}) {
		t.Error("falta el evento de tempo")
	}
	// Delta 480 (0x83 0x60): note-off de la primera nota y note-on inmediato de la segunda.
	if !bytes.Contains(b, []byte{0x83, 0x60, 0x80, 60, 0, 0x00, 0x90, 60, 100}) {
		t.Error("el note-off no precede al note-on en el mismo tick")
	}
	if !bytes.Contains(b, []byte{0x99, 36, 90}) {
		t.Error("la percusión debería ir por el canal 10")
	}
	if !bytes.HasSuffix(b, []byte{0x00, 0xFF, 0x2F, 0x00}) {
		t.Error("falta el fin de pista")
	}
}
