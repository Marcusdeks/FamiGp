import { describe, expect, it } from 'vitest';
import type { Preset, Song } from '../types';
import { createProject, emptyHistory, emptySong, fitLines, record, redo, replaceLine, toPayload, undo } from './project';

const note = (pitch: number) => ({ start: 0, duration: 480, pitch, velocity: 95 });

const song: Song = {
  title: 'Prueba',
  artist: '',
  tempos: [{ tick: 0, bpm: 120 }],
  bars: [0],
  length: 3840,
  tracks: [
    { name: 'Guitarra', percussion: false, notes: [note(60)] },
    { name: 'Batería', percussion: true, notes: [note(36)] },
  ],
  defaults: { nes: [0, -1, -1, 1] },
};

const nes: Preset = {
  id: 'nes',
  name: 'NES',
  description: '',
  voices: [
    { name: 'Pulso 1', drums: false },
    { name: 'Pulso 2', drums: false },
    { name: 'Triángulo', drums: false },
    { name: 'Ruido', drums: true },
  ],
};

describe('createProject', () => {
  it('copia las pistas propuestas en cada canal', () => {
    const project = createProject(song, nes, 1);
    expect(project.lines.map((l) => l.source)).toEqual([0, -1, -1, 1]);
    expect(project.lines[3].notes).toEqual([note(36)]);
    // Las notas son una copia: editar el proyecto no altera la canción original.
    expect(project.lines[0].notes).not.toBe(song.tracks[0].notes);
  });

  it('completa los canales que faltan sin tocar los existentes', () => {
    const lines = createProject(song, nes).lines.slice(0, 2);
    const fitted = fitLines(lines, song, nes);
    expect(fitted).toHaveLength(4);
    expect(fitted[0]).toBe(lines[0]);
    expect(fitted[3].notes).toEqual([]);
  });
});

describe('toPayload', () => {
  it('envía solo transposición y notas de cada voz del chip', () => {
    const project = createProject(song, nes);
    const payload = toPayload({ ...project, lines: replaceLine(project.lines, 0, { ...project.lines[0], transpose: 12 }) }, nes);
    expect(payload.preset).toBe('nes');
    expect(payload.lines[0]).toEqual({ transpose: 12, notes: [note(60)] });
  });

  it('silencia los canales muteados y, en modo solo, todos menos uno', () => {
    const project = createProject(song, nes);
    const muted = { ...project, lines: replaceLine(project.lines, 3, { ...project.lines[3], muted: true }) };
    expect(toPayload(muted, nes).lines.map((l) => l.notes.length)).toEqual([1, 0, 0, 0]);
    // El solo hace sonar el canal elegido aunque esté silenciado.
    expect(toPayload(muted, nes, 3).lines.map((l) => l.notes.length)).toEqual([0, 0, 0, 1]);
  });
});

describe('historial', () => {
  it('deshace y rehace en orden y descarta el rehacer al editar', () => {
    const a = { lines: [], song, channel: 0 };
    const b = { lines: [], song, channel: 1 };
    const c = { lines: [], song, channel: 2 };

    let history = record(record(emptyHistory, a), b); // estado actual: c
    const undone = undo(history, c);
    expect(undone?.[1]).toBe(b);
    history = undone![0];

    const redone = redo(history, b);
    expect(redone?.[1]).toBe(c);

    expect(record(history, b).future).toEqual([]);
    expect(undo(emptyHistory, a)).toBeNull();
  });
});

describe('emptySong', () => {
  it('crea una canción sin pistas con todos los canales vacíos', () => {
    const empty = emptySong([nes], 'Nuevo');
    expect(empty.tracks).toEqual([]);
    expect(empty.bars).toHaveLength(16);
    expect(empty.length).toBe(16 * 3840);
    const project = createProject(empty, nes);
    expect(project.lines.map((l) => l.notes.length)).toEqual([0, 0, 0, 0]);
  });
});
