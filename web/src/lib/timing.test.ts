import { describe, expect, it } from 'vitest';
import { PPQ } from '../types';
import { durationSeconds, formatPrecise, formatTime, secondsToTick, tickToSeconds } from './timing';

// Dos negras a 120 BPM (0,5 s cada una) y después a 60 BPM (1 s cada una).
const tempos = [
  { tick: 0, bpm: 120 },
  { tick: 2 * PPQ, bpm: 60 },
];

describe('timing', () => {
  it('calcula la duración con cambios de tempo', () => {
    expect(durationSeconds(tempos, 4 * PPQ)).toBe(3);
  });

  it('convierte entre segundos y ticks a ambos lados de un cambio de tempo', () => {
    expect(secondsToTick(tempos, 0.5)).toBe(PPQ);
    expect(secondsToTick(tempos, 2)).toBe(3 * PPQ);
    expect(tickToSeconds(tempos, PPQ)).toBe(0.5);
    expect(tickToSeconds(tempos, 3 * PPQ)).toBe(2);
  });

  it('formatea minutos y segundos', () => {
    expect(formatTime(59.6)).toBe('1:00');
    expect(formatTime(125)).toBe('2:05');
    expect(formatPrecise(65.34)).toBe('1:05,3');
    expect(formatPrecise(12)).toBe('0:12,0');
  });
});
