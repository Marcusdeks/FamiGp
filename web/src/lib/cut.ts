// Corte de un tramo de la canción: elimina el intervalo [from, to) en ticks
// y adelanta todo lo que viene después, como al cortar en un editor de audio.

import type { Note, Project, Tempo } from '../types';

// Recorta las notas: las que quedan dentro del tramo desaparecen, las
// posteriores se adelantan y las que cruzan un borde pierden la parte cortada
// (si lo cruzan entero, conservan ambos extremos unidos).
export function cutNotes(notes: Note[], from: number, to: number): Note[] {
  const length = to - from;
  return notes.flatMap((note) => {
    const end = note.start + note.duration;
    if (end <= from) return [note];
    if (note.start >= to) return [{ ...note, start: note.start - length }];
    const duration = Math.max(0, from - note.start) + Math.max(0, end - to);
    return duration > 0 ? [{ ...note, start: Math.min(note.start, from), duration }] : [];
  });
}

// El tempo vigente al final del tramo pasa a regir desde su inicio; los
// cambios posteriores se adelantan y se descartan los que no cambian nada.
export function cutTempos(tempos: Tempo[], from: number, to: number): Tempo[] {
  const length = to - from;
  const atEnd = tempos.findLast((t) => t.tick <= to) ?? tempos[0];
  const result = [
    ...tempos.filter((t) => t.tick < from),
    { tick: from, bpm: atEnd.bpm },
    ...tempos.filter((t) => t.tick > to).map((t) => ({ ...t, tick: t.tick - length })),
  ];
  return result.filter((t, i) => i === 0 || t.bpm !== result[i - 1].bpm);
}

// Los compases dentro del tramo desaparecen y los posteriores se adelantan;
// siempre queda un compás en el tick 0.
export function cutBars(bars: number[], from: number, to: number): number[] {
  const kept = [...bars.filter((b) => b < from), ...bars.filter((b) => b >= to).map((b) => b - (to - from))];
  return kept[0] === 0 ? kept : [0, ...kept];
}

// Aplica el corte a todo el proyecto: líneas de los canales, pistas de
// origen (para que copiarlas después siga cuadrando) y estructura temporal.
export function cutProject(project: Project, from: number, to: number): Project {
  if (!(from >= 0 && to > from)) throw new Error('Tramo de corte inválido');
  const { song } = project;
  return {
    ...project,
    lines: project.lines.map((line) => ({ ...line, notes: cutNotes(line.notes, from, to) })),
    song: {
      ...song,
      tempos: cutTempos(song.tempos, from, to),
      bars: cutBars(song.bars, from, to),
      length: song.length - Math.max(0, Math.min(to, song.length) - from),
      tracks: song.tracks.map((track) => ({ ...track, notes: cutNotes(track.notes, from, to) })),
    },
  };
}
