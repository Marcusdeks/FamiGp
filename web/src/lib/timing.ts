// Conversión entre ticks y segundos según el mapa de tempo.

import { PPQ, type Tempo } from '../types';

const secondsPerTick = (bpm: number) => 60 / (bpm * PPQ);

// Duración en segundos de una canción de `length` ticks.
export function durationSeconds(tempos: Tempo[], length: number): number {
  return tempos.reduce((total, t, i) => total + ((tempos[i + 1]?.tick ?? length) - t.tick) * secondsPerTick(t.bpm), 0);
}

// Segundo en que suena el tick indicado.
export function tickToSeconds(tempos: Tempo[], tick: number): number {
  return tempos.reduce((total, t, i) => {
    const end = Math.min(tick, tempos[i + 1]?.tick ?? Infinity);
    return end > t.tick ? total + (end - t.tick) * secondsPerTick(t.bpm) : total;
  }, 0);
}

// Tick que suena en el segundo indicado; sirve para situar el cursor de
// reproducción en el editor.
export function secondsToTick(tempos: Tempo[], seconds: number): number {
  let elapsed = 0;
  for (const [i, { tick, bpm }] of tempos.entries()) {
    const next = tempos[i + 1];
    const span = next ? (next.tick - tick) * secondsPerTick(bpm) : Infinity;
    if (seconds < elapsed + span) return tick + (seconds - elapsed) / secondsPerTick(bpm);
    elapsed += span;
  }
  return 0;
}

// Minutos, segundos y décimas con coma decimal: "1:05,3".
export function formatPrecise(seconds: number): string {
  const tenths = Math.round(seconds * 10);
  const rest = ((tenths % 600) / 10).toFixed(1).padStart(4, '0');
  return `${Math.floor(tenths / 600)}:${rest.replace('.', ',')}`;
}

export function formatTime(seconds: number): string {
  const total = Math.round(seconds);
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`;
}
