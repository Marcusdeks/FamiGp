package guitarpro

import (
	"bytes"
	"encoding/binary"
)

// Guitar Pro 6 (.gpx) empaqueta sus archivos en un sistema de ficheros
// propio (BCFS) dividido en sectores, normalmente comprimido (BCFZ).
const sectorSize = 0x1000

// parseGPX extrae score.gpif del contenedor de Guitar Pro 6 y lo interpreta.
func parseGPX(data []byte) (*score, error) {
	var fs []byte
	switch {
	case bytes.HasPrefix(data, []byte("BCFZ")):
		raw, err := decompressBCFZ(data[4:])
		if err != nil || !bytes.HasPrefix(raw, []byte("BCFS")) {
			return nil, ErrCorrupt
		}
		fs = raw[4:]
	case bytes.HasPrefix(data, []byte("BCFS")):
		fs = data[4:]
	default:
		return nil, ErrUnsupported
	}
	xmlData := findGPXFile(fs, "score.gpif")
	if xmlData == nil {
		return nil, ErrCorrupt
	}
	return parseGPIF(xmlData)
}

// decompressBCFZ implementa el LZ77 de Guitar Pro 6. Tras la longitud
// esperada (int32) viene un flujo de bits: un 0 introduce de 0 a 3 bytes
// literales; un 1, una referencia (distancia, tamaño) a datos ya escritos.
func decompressBCFZ(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, ErrCorrupt
	}
	size := int(int32(binary.LittleEndian.Uint32(data)))
	if size < 0 || size > maxXMLSize {
		return nil, ErrCorrupt
	}
	br := bitReader{data: data[4:]}
	out := make([]byte, 0, size)
	for len(out) < size && !br.eof() {
		if br.bits(1) == 1 {
			width := br.bits(4)
			distance, length := br.bitsReversed(width), br.bitsReversed(width)
			from := len(out) - distance
			if distance <= 0 || from < 0 {
				return nil, ErrCorrupt
			}
			out = append(out, out[from:from+min(distance, length)]...)
		} else {
			for n := br.bitsReversed(2); n > 0; n-- {
				out = append(out, byte(br.bits(8)))
			}
		}
	}
	return out, nil
}

// bitReader lee bits de mayor a menor peso dentro de cada byte. Al agotarse
// los datos devuelve ceros, como un flujo terminado en relleno.
type bitReader struct {
	data []byte
	pos  int // posición en bits
}

func (r *bitReader) eof() bool { return r.pos >= len(r.data)*8 }

func (r *bitReader) bit() int {
	if r.eof() {
		return 0
	}
	b := int(r.data[r.pos/8]>>(7-r.pos%8)) & 1
	r.pos++
	return b
}

// bits lee n bits con el primero como el más significativo.
func (r *bitReader) bits(n int) int {
	v := 0
	for range n {
		v = v<<1 | r.bit()
	}
	return v
}

// bitsReversed lee n bits con el primero como el menos significativo.
func (r *bitReader) bitsReversed(n int) int {
	v := 0
	for i := range n {
		v |= r.bit() << i
	}
	return v
}

// findGPXFile busca un archivo por nombre en el sistema BCFS. Cada sector
// que empieza por el entero 2 describe un archivo: nombre en 0x04, tamaño en
// 0x8C y, desde 0x94, la lista de sectores con sus datos terminada en 0.
func findGPXFile(fs []byte, name string) []byte {
	le := binary.LittleEndian
	for off := sectorSize; off+0x98 <= len(fs); off += sectorSize {
		if le.Uint32(fs[off:]) != 2 {
			continue
		}
		fileName, _, _ := bytes.Cut(fs[off+0x04:off+0x83], []byte{0})
		if string(fileName) != name {
			continue
		}
		size := int(le.Uint32(fs[off+0x8C:]))
		var content []byte
		for p := off + 0x94; p+4 <= len(fs) && len(content) < size; p += 4 {
			sector := int(le.Uint32(fs[p:])) * sectorSize
			if sector == 0 || sector+sectorSize > len(fs) {
				break
			}
			content = append(content, fs[sector:sector+sectorSize]...)
		}
		return content[:min(size, len(content))]
	}
	return nil
}
