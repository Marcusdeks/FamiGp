import { describe, expect, it } from 'vitest';
import type { Project } from '../types';
import { cutBars, cutNotes, cutProject, cutTempos } from './cut';

const note = (start: number, duration: number) => ({ start, duration, pitch: 60, velocity: 95 });

describe('cutNotes', () => {
  // Tramo cortado: [100, 200)
  it('conserva lo anterior, adelanta lo posterior y elimina lo de dentro', () => {
    expect(cutNotes([note(0, 50), note(250, 50), note(120, 30)], 100, 200)).toEqual([note(0, 50), note(150, 50)]);
  });

  it('recorta las notas que cruzan un borde', () => {
    expect(cutNotes([note(50, 100)], 100, 200)).toEqual([note(50, 50)]); // cruza el inicio
    expect(cutNotes([note(150, 100)], 100, 200)).toEqual([note(100, 50)]); // cruza el final
    expect(cutNotes([note(50, 200)], 100, 200)).toEqual([note(50, 100)]); // lo cruza entero
  });
});

describe('cutTempos', () => {
  const tempos = [
    { tick: 0, bpm: 120 },
    { tick: 150, bpm: 90 },
    { tick: 300, bpm: 60 },
  ];

  it('aplica desde el corte el tempo vigente al final del tramo', () => {
    expect(cutTempos(tempos, 100, 200)).toEqual([
      { tick: 0, bpm: 120 },
      { tick: 100, bpm: 90 },
      { tick: 200, bpm: 60 },
    ]);
  });

  it('al cortar el principio mantiene un tempo en el tick 0 y quita los redundantes', () => {
    expect(cutTempos(tempos, 0, 200)).toEqual([
      { tick: 0, bpm: 90 },
      { tick: 100, bpm: 60 },
    ]);
    expect(cutTempos([{ tick: 0, bpm: 120 }], 0, 50)).toEqual([{ tick: 0, bpm: 120 }]);
  });
});

describe('cutBars', () => {
  it('adelanta los compases y garantiza uno en el tick 0', () => {
    expect(cutBars([0, 100, 200, 300], 100, 200)).toEqual([0, 100, 200]);
    expect(cutBars([0, 100, 200], 0, 150)).toEqual([0, 50]);
  });
});

describe('cutProject', () => {
  const project: Project = {
    id: 'p',
    name: 'Prueba',
    preset: 'nes',
    created: 0,
    updated: 0,
    lines: [{ source: 0, transpose: 0, notes: [note(0, 100), note(300, 100)] }],
    song: {
      title: 'Prueba',
      artist: '',
      tempos: [{ tick: 0, bpm: 120 }],
      bars: [0, 200, 400],
      length: 600,
      tracks: [{ name: 'Guitarra', percussion: false, notes: [note(300, 100)] }],
      defaults: {},
    },
  };

  it('corta líneas, pistas de origen y duración a la vez', () => {
    const cut = cutProject(project, 100, 300);
    expect(cut.lines[0].notes).toEqual([note(0, 100), note(100, 100)]);
    expect(cut.song.tracks[0].notes).toEqual([note(100, 100)]);
    expect(cut.song.length).toBe(400);
    expect(cut.song.bars).toEqual([0, 200]);
    // El proyecto original no se modifica (inmutabilidad para deshacer).
    expect(project.lines[0].notes).toEqual([note(0, 100), note(300, 100)]);
  });

  it('rechaza tramos inválidos', () => {
    expect(() => cutProject(project, 200, 100)).toThrow();
  });
});
