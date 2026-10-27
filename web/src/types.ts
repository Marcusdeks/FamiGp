// Tipos del modelo compartido con el servidor (paquete song de Go) y del
// estado de los proyectos en el cliente.

export const PPQ = 960; // ticks por negra, igual que el servidor

export interface Note {
  start: number; // ticks
  duration: number; // ticks
  pitch: number; // altura MIDI; en percusión, instrumento General MIDI
  velocity: number; // 1-127
}

export interface Tempo {
  tick: number;
  bpm: number;
}

export interface Track {
  name: string;
  percussion: boolean;
  notes: Note[];
}

export interface Song {
  title: string;
  artist: string;
  tempos: Tempo[];
  bars: number[]; // tick de inicio de cada compás
  length: number; // ticks
  tracks: Track[];
  defaults: Record<string, number[]>; // pista propuesta para cada voz, por chip
}

export interface Voice {
  name: string;
  drums: boolean;
}

export interface Preset {
  id: string;
  name: string;
  description: string;
  voices: Voice[];
}

// Line es lo que toca un canal del chip: sus notas, la pista de la que se
// copiaron (-1 si ninguna), una transposición en semitonos que se aplica al
// sintetizar sin modificar las notas y si está silenciado.
export interface Line {
  source: number;
  transpose: number;
  notes: Note[];
  muted?: boolean; // opcional: los proyectos guardados antes de existir no lo tienen
}

export interface Project {
  id: string;
  name: string;
  song: Song;
  preset: string;
  lines: Line[];
  created: number;
  updated: number;
}

// ProjectPayload es el cuerpo que esperan /api/render y /api/midi.
export interface ProjectPayload {
  title: string;
  preset: string;
  tempos: Tempo[];
  length: number;
  lines: Pick<Line, 'transpose' | 'notes'>[];
}
