package chip

import (
	"encoding/binary"
	"io"
	"math"
)

// WriteWAV codifica audio mono en [-1, 1] como WAV PCM de 16 bits a SampleRate.
func WriteWAV(w io.Writer, samples []float32) error {
	const headerSize, bytesPerSample = 44, 2
	dataSize := len(samples) * bytesPerSample
	le := binary.LittleEndian

	b := make([]byte, 0, headerSize+dataSize)
	b = append(b, "RIFF"...)
	b = le.AppendUint32(b, uint32(headerSize-8+dataSize))
	b = append(b, "WAVEfmt "...)
	b = le.AppendUint32(b, 16) // tamaño del bloque fmt
	b = le.AppendUint16(b, 1)  // PCM sin compresión
	b = le.AppendUint16(b, 1)  // mono
	b = le.AppendUint32(b, SampleRate)
	b = le.AppendUint32(b, SampleRate*bytesPerSample) // bytes por segundo
	b = le.AppendUint16(b, bytesPerSample)            // bytes por bloque
	b = le.AppendUint16(b, 8*bytesPerSample)          // bits por muestra
	b = append(b, "data"...)
	b = le.AppendUint32(b, uint32(dataSize))
	for _, s := range samples {
		b = le.AppendUint16(b, uint16(int16(math.Round(float64(max(-1, min(1, s)))*math.MaxInt16))))
	}
	_, err := w.Write(b)
	return err
}
