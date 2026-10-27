package guitarpro

import (
	"encoding/binary"
	"math/bits"
	"strings"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

// reader lee los tipos primitivos (little-endian) de los formatos binarios.
// El error es persistente: tras el primer fallo todas las lecturas devuelven
// ceros, de modo que el análisis termina limpiamente sin comprobar cada paso.
type reader struct {
	data []byte
	pos  int
	err  error
}

var zeros [8]byte

func (r *reader) next(n int) []byte {
	if r.err == nil && (n < 0 || n > len(r.data)-r.pos) {
		r.err = ErrCorrupt
	}
	if r.err != nil {
		if n >= 0 && n <= len(zeros) {
			return zeros[:n]
		}
		return nil
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

func (r *reader) skip(n int) { r.next(n) }
func (r *reader) u8() int    { return int(r.next(1)[0]) }
func (r *reader) i8() int    { return int(int8(r.next(1)[0])) }
func (r *reader) i16() int   { return int(int16(binary.LittleEndian.Uint16(r.next(2)))) }
func (r *reader) i32() int   { return int(int32(binary.LittleEndian.Uint32(r.next(4)))) }

// byteString lee una cadena de longitud indicada por un byte, almacenada en
// un campo de tamaño fijo size.
func (r *reader) byteString(size int) string {
	n := r.u8()
	b := r.next(size)
	return latin1(b[:min(n, len(b))])
}

// intByteString lee una cadena precedida por su tamaño total (int32) y su
// longitud real (byte), el formato más usado en los metadatos.
func (r *reader) intByteString() string {
	return r.byteString(r.i32() - 1)
}

// intString lee una cadena precedida únicamente por su longitud en int32.
func (r *reader) intString() string {
	return latin1(r.next(r.i32()))
}

// latin1 decodifica texto Windows-1252/Latin-1, la codificación de los
// archivos antiguos, para conservar tildes y eñes.
func latin1(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		sb.WriteRune(rune(c))
	}
	return sb.String()
}

// binaryParser mantiene el estado necesario mientras se recorren los compases.
type binaryParser struct {
	reader
	version int // 300, 400, 406, 500 o 510
	sc      *score
	starts  []int
}

// parseBinary interpreta los formatos GP3, GP4 y GP5. El orden de lectura es
// estrictamente secuencial: cualquier campo no utilizado debe saltarse
// igualmente para no perder la alineación.
func parseBinary(data []byte) (*score, error) {
	p := &binaryParser{reader: reader{data: data}, sc: &score{}}
	header := p.byteString(30)
	if p.err != nil {
		return nil, p.err
	}
	if len(header) < 4 {
		return nil, ErrUnsupported
	}
	v := header[len(header)-4:] // "5.10"
	p.version = int(v[0]-'0')*100 + int(v[2]-'0')*10 + int(v[3]-'0')
	if p.version < 300 || p.version > 510 {
		return nil, ErrUnsupported
	}
	p.info()
	trackCount := p.songSettings()
	p.bars(trackCount)
	tunings := p.tracks(trackCount)
	p.measures(tunings)
	if p.err != nil {
		return nil, p.err
	}
	return p.sc, nil
}

// info lee los metadatos de la canción; solo se conservan título y artista.
func (p *binaryParser) info() {
	p.sc.title = p.intByteString()
	p.intByteString() // subtítulo
	p.sc.artist = p.intByteString()
	fields := 5 // álbum, letra, copyright, tablatura, instrucciones
	if p.version >= 500 {
		fields++ // GP5 separa autor de letra y de música
	}
	for range fields {
		p.intByteString()
	}
	for n := p.i32(); n > 0 && p.err == nil; n-- {
		p.intByteString() // líneas de notas
	}
}

// songSettings recorre los ajustes globales (letras, página, tempo, canales
// MIDI) y devuelve el número de pistas tras dejar listo el de compases.
func (p *binaryParser) songSettings() int {
	if p.version < 500 {
		p.skip(1) // triplet feel global
	}
	if p.version >= 400 {
		p.skip(4) // pista asociada a la letra
		for range 5 {
			p.skip(4)
			p.intString()
		}
	}
	if p.version >= 510 {
		p.skip(19) // efecto maestro RSE
	}
	if p.version >= 500 {
		p.skip(30) // configuración de página
		for range 11 {
			p.intByteString() // plantillas de cabecera y nombre de tempo
		}
	}
	p.sc.tempo = float64(p.i32())
	if p.version >= 510 {
		p.skip(1) // ocultar tempo
	}
	if p.version >= 400 {
		p.skip(5) // tonalidad y octava
	} else {
		p.skip(4)
	}
	p.skip(64 * 12) // tabla de 64 canales MIDI
	if p.version >= 500 {
		p.skip(42) // direcciones (coda, segno...) y reverb maestra
	}
	bars, tracks := p.i32(), p.i32()
	if bars <= 0 || bars > maxBars || tracks <= 0 || tracks > maxTracks || p.sc.tempo <= 0 {
		p.err = ErrCorrupt
	}
	// Con el error activo los recuentos no son fiables: no se reserva nada.
	if p.err != nil {
		return 0
	}
	p.sc.bars = make([]masterBar, bars)
	return tracks
}

// bars lee las cabeceras de compás: indicación de compás y repeticiones.
func (p *binaryParser) bars(trackCount int) {
	prev := masterBar{num: 4, den: 4}
	for i := range p.sc.bars {
		if p.version >= 500 && i > 0 {
			p.skip(1)
		}
		b := masterBar{num: prev.num, den: prev.den}
		flags := p.u8()
		if flags&0x01 != 0 {
			b.num = p.i8()
		}
		if flags&0x02 != 0 {
			b.den = p.i8()
		}
		b.repeatStart = flags&0x04 != 0
		if flags&0x08 != 0 {
			// GP5 guarda el total de pasadas; GP3/GP4, el número de repeticiones.
			b.repeatPlays = p.i8()
			if p.version < 500 {
				b.repeatPlays++
			}
		}
		if p.version < 500 && flags&0x10 != 0 {
			b.alternates = p.legacyAlternate(i)
		}
		if flags&0x20 != 0 {
			p.intByteString() // marcador
			p.skip(4)         // color del marcador
		}
		if flags&0x40 != 0 {
			p.skip(2) // tonalidad
		}
		if p.version >= 500 {
			if flags&0x10 != 0 {
				b.alternates = p.u8()
			}
			if flags&0x03 != 0 {
				p.skip(4) // agrupación de plicas
			}
			if flags&0x10 == 0 {
				p.skip(1)
			}
			p.skip(1) // triplet feel
		}
		if !b.valid() {
			p.err = ErrCorrupt
			return
		}
		p.sc.bars[i] = b
		prev = b
	}
	p.starts = p.sc.barStarts()
	p.sc.tracks = make([]*track, 0, trackCount)
}

// legacyAlternate convierte el número de final alternativo de GP3/GP4 en una
// máscara, descontando los finales ya usados desde el inicio de la repetición.
func (p *binaryParser) legacyAlternate(bar int) int {
	value := p.u8()
	used := 0
	for j := bar - 1; j >= 0 && !p.sc.bars[j].repeatStart; j-- {
		used |= p.sc.bars[j].alternates
	}
	return (1<<value - 1) ^ used
}

// tracks lee la definición de cada pista y devuelve su afinación efectiva
// (afinación de cada cuerda más la cejilla), de la cuerda más aguda a la más grave.
func (p *binaryParser) tracks(count int) [][]int {
	tunings := make([][]int, count)
	for i := range count {
		if p.version >= 500 && (i == 0 || p.version == 500) {
			p.skip(1)
		}
		flags := p.u8()
		name := p.byteString(40)
		strings := min(max(p.i32(), 0), 7)
		tuning := make([]int, 7)
		for s := range tuning {
			tuning[s] = p.i32()
		}
		p.skip(4) // puerto MIDI
		channel := p.i32() - 1
		p.skip(8) // canal de efectos y número de trastes
		capo := p.i32()
		p.skip(4) // color
		if p.version >= 500 {
			p.skip(29) // ajustes de pista y RSE
			if p.version == 500 {
				p.skip(15)
			} else {
				p.skip(20) // instrumento RSE y ecualizador
				p.intByteString()
				p.intByteString()
			}
		}
		for s := range strings {
			tuning[s] += capo
		}
		tunings[i] = tuning[:strings]
		p.sc.tracks = append(p.sc.tracks, newTrack(name, flags&0x01 != 0 || channel%16 == 9, len(p.sc.bars)))
	}
	if p.version >= 500 {
		if p.version == 500 {
			p.skip(2)
		} else {
			p.skip(1)
		}
	}
	return tunings
}

// measures lee el contenido de cada compás de cada pista (GP5 tiene dos voces).
func (p *binaryParser) measures(tunings [][]int) {
	voices := 1
	if p.version >= 500 {
		voices = 2
	}
	for bar := range p.sc.bars {
		for ti, t := range p.sc.tracks {
			for voice := range voices {
				offset := 0
				for n := p.i32(); n > 0 && p.err == nil; n-- {
					offset += p.beat(t, tunings[ti], bar, voice, offset)
				}
			}
			// Salto de línea; el último compás del archivo puede omitirlo.
			if p.version >= 500 && p.pos < len(p.data) {
				p.skip(1)
			}
		}
	}
}

// beat lee un pulso (nota, acorde o silencio) y devuelve su duración en ticks.
func (p *binaryParser) beat(t *track, tuning []int, bar, voice, offset int) int {
	flags := p.u8()
	status := 1 // 0 = vacío, 1 = normal, 2 = silencio
	if flags&0x40 != 0 {
		status = p.u8()
	}
	duration := p.duration(flags&0x01 != 0)
	if flags&0x20 != 0 {
		duration = tuplet(duration, p.i32())
	}
	if flags&0x02 != 0 {
		p.chord()
	}
	if flags&0x04 != 0 {
		p.intByteString() // texto
	}
	if flags&0x08 != 0 {
		p.beatEffects()
	}
	if flags&0x10 != 0 {
		if bpm := p.mixTable(); bpm > 0 {
			p.sc.bars[bar].tempos = append(p.sc.bars[bar].tempos, tempoMark{offset, float64(bpm)})
		}
	}
	// Un bit por cuerda: el bit 6 es la cuerda más aguda.
	stringFlags := p.u8()
	for s := range tuning {
		if stringFlags&(1<<(6-s)) != 0 {
			p.note(t, tuning[s], bar, voice<<8|s, offset, duration)
		}
	}
	if p.version >= 500 && p.i16()&0x0800 != 0 {
		p.skip(1)
	}
	if status == 0 {
		return 0
	}
	return duration
}

// duration convierte la figura codificada (-2 = redonda ... 4 = semifusa) a ticks.
func (p *binaryParser) duration(dotted bool) int {
	v := p.i8()
	if v < -2 || v > 6 {
		p.err = ErrCorrupt
		return 0
	}
	d := song.PPQ * 4 >> (v + 2)
	if dotted {
		d += d / 2
	}
	return d
}

// tuplet ajusta la duración de un grupo irregular: n notas en el tiempo de
// la potencia de dos inmediatamente inferior (tresillo = 3:2, cinquillo = 5:4...).
func tuplet(d, n int) int {
	if n < 3 {
		return d
	}
	return d * (1 << (bits.Len(uint(n-1)) - 1)) / n
}

// note lee una nota y la añade a la pista. Las notas muertas se descartan
// porque no tienen altura musical.
func (p *binaryParser) note(t *track, base, bar, key, offset, duration int) {
	flags := p.u8()
	kind := 1 // 1 = normal, 2 = ligada, 3 = muerta
	if flags&0x20 != 0 {
		kind = p.u8()
	}
	if flags&0x01 != 0 && p.version < 500 {
		p.skip(2) // duración independiente (obsoleta)
	}
	velocity := defaultVelocity
	if flags&0x10 != 0 {
		velocity = min(max(15+16*(p.i8()-1), 1), 127)
	}
	fret := 0
	if flags&0x20 != 0 {
		fret = p.i8()
	}
	if flags&0x80 != 0 {
		p.skip(2) // digitación
	}
	if p.version >= 500 {
		if flags&0x01 != 0 {
			p.skip(8) // porcentaje de duración (float64)
		}
		p.skip(1)
	}
	if flags&0x08 != 0 {
		p.noteEffects()
	}
	if kind != 3 {
		t.add(p.starts, bar, key, note{offset, duration, base + fret, velocity}, kind == 2)
	}
}

// chord salta un diagrama de acorde: no aporta notas, solo representación gráfica.
func (p *binaryParser) chord() {
	switch {
	case p.version >= 500:
		p.skip(107)
	case p.u8() != 0: // formato nuevo
		if p.version >= 400 {
			p.skip(106)
		} else {
			p.skip(124)
		}
	default: // formato antiguo
		p.intByteString()
		if p.i32() > 0 {
			if p.version >= 406 {
				p.skip(7 * 4)
			} else {
				p.skip(6 * 4)
			}
		}
	}
}

// beatEffects salta los efectos de pulso (rasgueos, palanca, slap...).
func (p *binaryParser) beatEffects() {
	f1, f2 := p.u8(), 0
	if p.version >= 400 {
		f2 = p.u8()
	}
	if f1&0x20 != 0 {
		p.skip(1)
		if p.version < 400 {
			p.skip(4)
		}
	}
	if f2&0x04 != 0 {
		p.bend()
	}
	if f1&0x40 != 0 {
		p.skip(2) // rasgueo arriba/abajo
	}
	if f2&0x02 != 0 {
		p.skip(1) // dirección de púa
	}
}

// noteEffects salta los efectos de nota (bends, notas de adorno, armónicos...).
func (p *binaryParser) noteEffects() {
	f1, f2 := p.u8(), 0
	if p.version >= 400 {
		f2 = p.u8()
	}
	if f1&0x01 != 0 {
		p.bend()
	}
	if f1&0x10 != 0 {
		p.skip(4) // nota de adorno
		if p.version >= 500 {
			p.skip(1)
		}
	}
	if f2&0x04 != 0 {
		p.skip(1) // trémolo de púa
	}
	if f2&0x08 != 0 {
		p.skip(1) // slide
	}
	if f2&0x10 != 0 {
		harmonic := p.u8()
		if p.version >= 500 {
			switch harmonic {
			case 2: // artificial
				p.skip(3)
			case 3: // tapped
				p.skip(1)
			}
		}
	}
	if f2&0x20 != 0 {
		p.skip(2) // trino
	}
}

// bend salta una curva de bend o de palanca: cabecera y lista de puntos.
func (p *binaryParser) bend() {
	p.skip(5)
	for n := p.i32(); n > 0 && p.err == nil; n-- {
		p.skip(9)
	}
}

// mixTable lee un cambio de mezcla y devuelve el nuevo tempo (o -1 si no cambia).
func (p *binaryParser) mixTable() int {
	p.skip(1) // instrumento
	if p.version >= 500 {
		p.skip(16) // instrumento RSE
	}
	values := p.next(6) // volumen, panorama, chorus, reverb, phaser, trémolo
	if p.version >= 500 {
		p.intByteString() // nombre del tempo
	}
	tempo := p.i32()
	// Cada valor modificado (>= 0) va seguido de la duración de su transición.
	for _, v := range values {
		if int8(v) >= 0 {
			p.skip(1)
		}
	}
	if tempo >= 0 {
		p.skip(1)
		if p.version >= 510 {
			p.skip(1)
		}
	}
	if p.version >= 400 {
		p.skip(1) // aplicar a todas las pistas
	}
	if p.version >= 500 {
		p.skip(1) // wah
		if p.version >= 510 {
			p.intByteString()
			p.intByteString()
		}
	}
	return tempo
}
