// Operaciones puras sobre proyectos y su historial de cambios. No tocan el
// DOM ni el almacenamiento, de modo que se prueban de forma aislada.

import { PPQ, type Line, type Preset, type Project, type ProjectPayload, type Song } from '../types';

export const MAX_PROJECTS = 5;
export const CHANNEL_COLORS = ['#e4304c', '#f8b800', '#3cbcfc', '#7ee787'];
export const MAX_TRANSPOSE = 24; // semitonos en cada sentido
const UNDO_LIMIT = 100;

// Línea con una copia de las notas de la pista indicada (-1 = vacía).
export function newLine(song: Song, track: number): Line {
  return { source: track, transpose: 0, notes: track >= 0 ? structuredClone(song.tracks[track].notes) : [] };
}

// Completa las líneas hasta el número de voces del chip sin tocar las existentes.
export function fitLines(lines: Line[], song: Song, preset: Preset): Line[] {
  const missing = preset.voices.length - lines.length;
  return missing > 0 ? [...lines, ...Array.from({ length: missing }, () => newLine(song, -1))] : lines;
}

const EMPTY_BARS = 16;

// Canción vacía para componer desde cero: 16 compases de 4/4 a 120 BPM, sin
// pistas de origen y con todos los canales vacíos en cualquier chip.
export function emptySong(presets: Preset[], title: string): Song {
  const bar = 4 * PPQ;
  return {
    title,
    artist: '',
    tempos: [{ tick: 0, bpm: 120 }],
    bars: Array.from({ length: EMPTY_BARS }, (_, i) => i * bar),
    length: EMPTY_BARS * bar,
    tracks: [],
    defaults: Object.fromEntries(presets.map((p) => [p.id, p.voices.map(() => -1)])),
  };
}

export function createProject(song: Song, preset: Preset, now = Date.now()): Project {
  const lines = (song.defaults[preset.id] ?? []).map((track) => newLine(song, track));
  return {
    id: `${now}-${Math.random().toString(36).slice(2)}`,
    name: song.title,
    song,
    preset: preset.id,
    lines: fitLines(lines, song, preset),
    created: now,
    updated: now,
  };
}

export function replaceLine(lines: Line[], index: number, line: Line): Line[] {
  return lines.map((current, i) => (i === index ? line : current));
}

// Cuerpo para sintetizar o exportar. Los canales silenciados se envían sin
// notas; con `solo`, solo suena ese canal (aunque esté silenciado).
export function toPayload(project: Project, preset: Preset, solo?: number): ProjectPayload {
  return {
    title: project.name,
    preset: preset.id,
    tempos: project.song.tempos,
    length: project.song.length,
    lines: project.lines.slice(0, preset.voices.length).map(({ transpose, notes, muted }, i) => ({
      transpose,
      notes: (solo === undefined ? !muted : i === solo) ? notes : [],
    })),
  };
}

// --- Historial ---------------------------------------------------------------
// Cada instantánea guarda las líneas y la canción (lo que no cambia se
// comparte por referencia, así que es barato) y el canal editado, para
// volver a él. La canción hace falta porque un corte cambia su estructura.

export interface Snapshot {
  lines: Line[];
  song: Song;
  channel: number;
}

export interface History {
  past: Snapshot[];
  future: Snapshot[];
}

export const emptyHistory: History = { past: [], future: [] };

// Registra el estado previo a un cambio; cualquier rehacer pendiente se pierde.
export function record(history: History, snapshot: Snapshot): History {
  return { past: [...history.past, snapshot].slice(-UNDO_LIMIT), future: [] };
}

// Deshace: devuelve el nuevo historial y la instantánea a restaurar, o null
// si no hay nada que deshacer. `current` pasa a la pila de rehacer.
export function undo(history: History, current: Snapshot): [History, Snapshot] | null {
  const snapshot = history.past.at(-1);
  if (!snapshot) return null;
  return [{ past: history.past.slice(0, -1), future: [...history.future, current] }, snapshot];
}

export function redo(history: History, current: Snapshot): [History, Snapshot] | null {
  const snapshot = history.future.at(-1);
  if (!snapshot) return null;
  return [{ past: [...history.past, current], future: history.future.slice(0, -1) }, snapshot];
}
