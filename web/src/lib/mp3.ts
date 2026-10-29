// Conversión de WAV a MP3 con lamejs (versión en JavaScript del codificador
// LAME). Es lógica pura: se ejecuta dentro de un Web Worker (mp3.worker.ts)
// para que la interfaz no se congele mientras codifica.

import { Mp3Encoder } from '@breezystack/lamejs';

const KBPS = 192; // calidad alta: las ondas cuadradas tienen muchos armónicos agudos
const BLOCK = 1152 * 64; // múltiplo de las 1152 muestras de una trama MP3

interface Pcm {
  samples: Int16Array;
  sampleRate: number;
  channels: number;
}

// Lee un WAV PCM de 16 bits. Recorre los bloques RIFF en lugar de suponer una
// cabecera fija de 44 bytes, así acepta WAV con bloques extra (LIST, etc.).
export function readWav(buffer: ArrayBuffer): Pcm {
  const view = new DataView(buffer);
  const id = (offset: number) => String.fromCharCode(...new Uint8Array(buffer, offset, 4));
  if (buffer.byteLength < 12 || id(0) !== 'RIFF' || id(8) !== 'WAVE') throw new Error('El audio no es un WAV válido');

  let channels = 0;
  let sampleRate = 0;
  let bits = 0;
  for (let offset = 12; offset + 8 <= buffer.byteLength; ) {
    const size = view.getUint32(offset + 4, true);
    const body = offset + 8;
    if (id(offset) === 'fmt ') {
      channels = view.getUint16(body + 2, true);
      sampleRate = view.getUint32(body + 4, true);
      bits = view.getUint16(body + 14, true);
    } else if (id(offset) === 'data') {
      if (bits !== 16) throw new Error('Solo se admite WAV PCM de 16 bits');
      // slice copia los datos a un búfer propio, alineado como exige Int16Array.
      return { samples: new Int16Array(buffer.slice(body, body + size)), sampleRate, channels };
    }
    offset = body + size + (size % 2); // los bloques RIFF se alinean a 2 bytes
  }
  throw new Error('El WAV no contiene audio');
}

// Codifica un WAV mono a MP3 por bloques y devuelve el archivo completo.
export function wavToMp3(buffer: ArrayBuffer): Blob {
  const { samples, sampleRate, channels } = readWav(buffer);
  if (channels !== 1) throw new Error('Solo se admite audio mono');
  const encoder = new Mp3Encoder(1, sampleRate, KBPS);
  const parts: BlobPart[] = [];
  for (let i = 0; i < samples.length; i += BLOCK) {
    parts.push(new Uint8Array(encoder.encodeBuffer(samples.subarray(i, i + BLOCK))));
  }
  parts.push(new Uint8Array(encoder.flush()));
  return new Blob(parts, { type: 'audio/mpeg' });
}
