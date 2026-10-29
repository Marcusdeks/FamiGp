import { describe, expect, it } from 'vitest';
import { readWav, wavToMp3 } from './mp3';

// Construye un WAV PCM de 16 bits con un bloque extra (LIST) antes de los
// datos, para comprobar que el lector no supone una cabecera fija.
function wav(samples: number[], sampleRate = 44100, channels = 1): ArrayBuffer {
  const extra = 10; // bloque LIST de tamaño par
  const buffer = new ArrayBuffer(12 + 24 + 8 + extra + 8 + samples.length * 2);
  const view = new DataView(buffer);
  const text = (offset: number, s: string) => [...s].forEach((c, i) => view.setUint8(offset + i, c.charCodeAt(0)));
  text(0, 'RIFF');
  view.setUint32(4, buffer.byteLength - 8, true);
  text(8, 'WAVE');
  text(12, 'fmt ');
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true); // PCM
  view.setUint16(22, channels, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2 * channels, true);
  view.setUint16(32, 2 * channels, true);
  view.setUint16(34, 16, true);
  text(36, 'LIST');
  view.setUint32(40, extra, true);
  const data = 44 + extra;
  text(data, 'data');
  view.setUint32(data + 4, samples.length * 2, true);
  samples.forEach((s, i) => view.setInt16(data + 8 + i * 2, s, true));
  return buffer;
}

const tone = (seconds: number) => Array.from({ length: 44100 * seconds }, (_, i) => Math.round(Math.sin((2 * Math.PI * 440 * i) / 44100) * 12000));

describe('readWav', () => {
  it('lee las muestras saltando bloques desconocidos', () => {
    const pcm = readWav(wav([1, -2, 3]));
    expect(Array.from(pcm.samples)).toEqual([1, -2, 3]);
    expect(pcm.sampleRate).toBe(44100);
    expect(pcm.channels).toBe(1);
  });

  it('rechaza lo que no es un WAV', () => {
    expect(() => readWav(new TextEncoder().encode('no es audio').buffer)).toThrow('WAV');
  });
});

describe('wavToMp3', () => {
  it('genera un MP3 válido y mucho más pequeño que el WAV', async () => {
    const input = wav(tone(1));
    const mp3 = new Uint8Array(await wavToMp3(input).arrayBuffer());
    // Toda trama MP3 empieza con 11 bits a 1 (sincronización).
    expect(mp3[0]).toBe(0xff);
    expect(mp3[1] & 0xe0).toBe(0xe0);
    expect(mp3.length).toBeLessThan(input.byteLength / 3);
  });

  it('rechaza audio estéreo', () => {
    expect(() => wavToMp3(wav([0, 0], 44100, 2))).toThrow('mono');
  });
});
