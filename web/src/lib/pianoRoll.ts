// Editor de piano roll: dibuja en un canvas las notas de un canal y permite
// añadirlas, moverlas, alargarlas y borrarlas con el ratón.
//
// El canvas solo ocupa la parte visible: un elemento espaciador da al
// contenedor el tamaño de la canción completa para que el navegador gestione
// el desplazamiento, y cada fotograma se dibuja la zona que queda a la vista.
//
// Es una clase imperativa (el canvas no encaja en el renderizado declarativo
// de React) que trabaja sobre su propia copia de las notas: durante un gesto
// la modifica directamente y, al terminarlo, entrega un array nuevo mediante
// onChange, sin mutar nunca el estado de React.

import { PPQ, type Note, type Tempo } from '../types';
import { tickToSeconds } from './timing';

const KEYS_WIDTH = 100; // columna del teclado o de los nombres de percusión
const RULER_HEIGHT = 22; // regla con los números de compás
const ROW_HEIGHT = 14;
const EDGE = 8; // anchura máxima en píxeles del asa de cada extremo de una nota
const DRAG_THRESHOLD = 3; // píxeles que hay que mover el ratón para que cuente como arrastre
const PADDING_BARS = 8; // compases vacíos tras el final para seguir escribiendo
const MIN_ZOOM = 8 / PPQ; // píxeles por tick
const MAX_ZOOM = 400 / PPQ;
const DEFAULT_VELOCITY = 95;

const NOTE_NAMES = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B'];
const BLACK_KEYS = new Set([1, 3, 6, 8, 10]);

// Instrumentos General MIDI que se ofrecen siempre en el canal de ruido.
const DRUM_NAMES = new Map([
  [35, 'Bombo 2'], [36, 'Bombo'], [37, 'Aro'], [38, 'Caja'], [39, 'Palmas'], [40, 'Caja 2'],
  [41, 'Tom suelo'], [42, 'Charles'], [43, 'Tom suelo 2'], [44, 'Charles pie'], [45, 'Tom bajo'],
  [46, 'Charles abierto'], [47, 'Tom medio'], [48, 'Tom alto'], [49, 'Crash'], [50, 'Tom agudo'],
  [51, 'Ride'], [52, 'China'], [53, 'Campana ride'], [55, 'Splash'], [57, 'Crash 2'], [59, 'Ride 2'],
]);

const COLORS = {
  background: '#0e0b1f',
  whiteRow: '#1b1740',
  blackRow: '#141033',
  octave: '#3a3370',
  beat: '#28224f',
  bar: '#5a5299',
  subdivision: '#1f1a46',
  ghost: 'rgba(169, 162, 214, 0.22)',
  pastEnd: 'rgba(0, 0, 0, 0.35)',
  playhead: '#ffffff',
  text: '#f1ecff',
  muted: '#a9a2d6',
  whiteKey: '#e9e6f7',
  blackKey: '#2a2550',
};

const clamp = (value: number, low: number, high: number) => Math.min(high, Math.max(low, value));

export function noteName(pitch: number): string {
  return `${NOTE_NAMES[pitch % 12]}${Math.floor(pitch / 12) - 1}`;
}

export interface RollView {
  notes: Note[]; // notas guardadas del canal
  transpose: number; // semitonos; las notas se muestran a su altura audible
  drums: boolean;
  color: string;
  ghosts: Note[]; // notas de otros canales, ya transpuestas, como guía
  bars: number[];
  length: number;
  tempos: Tempo[]; // para mostrar duraciones en segundos
}

export interface RollCallbacks {
  onChange(notes: Note[]): void; // al terminar cada gesto, con las notas resultantes
  onPreview(pitch: number): void; // para oír una nota (altura audible) al tocarla
}

interface Drag {
  mode: 'move' | 'resize-start' | 'resize-end';
  note: Note;
  created: boolean;
  moved: boolean;
  startX: number;
  startY: number;
  startTick: number;
  origin: { start: number; pitch: number; end: number };
}

export class PianoRoll {
  #scroller: HTMLElement;
  #spacer: HTMLElement;
  #canvas: HTMLCanvasElement;
  #ctx: CanvasRenderingContext2D;
  #callbacks: RollCallbacks;
  #abort = new AbortController();
  #observer: ResizeObserver;

  #view: RollView | null = null;
  #notes: Note[] = []; // copia de trabajo
  #emitted: Note[] | null = null; // último array entregado con onChange
  #rows: number[] = []; // alturas audibles de arriba abajo
  #rowOf = new Map<number, number>(); // altura audible → fila
  #zoom = 48 / PPQ;
  #snap = PPQ / 4;
  #noteLength = PPQ / 2; // duración de las notas nuevas: la última usada
  #selected: Note | null = null;
  #hover: Note | null = null; // nota bajo el ratón, para mostrar sus asas
  #drag: Drag | null = null;
  #playhead: number | null = null;
  #frame = 0;

  constructor(scroller: HTMLElement, spacer: HTMLElement, canvas: HTMLCanvasElement, callbacks: RollCallbacks) {
    this.#scroller = scroller;
    this.#spacer = spacer;
    this.#canvas = canvas;
    this.#ctx = canvas.getContext('2d')!;
    this.#callbacks = callbacks;

    const { signal } = this.#abort;
    scroller.addEventListener('scroll', () => this.draw(), { signal });
    scroller.addEventListener('keydown', (e) => this.#onKey(e), { signal });
    scroller.addEventListener('wheel', (e) => this.#onWheel(e), { signal, passive: false });
    canvas.addEventListener('pointerdown', (e) => this.#onPointerDown(e), { signal });
    canvas.addEventListener('pointermove', (e) => this.#onPointerMove(e), { signal });
    canvas.addEventListener('pointerup', () => this.#onPointerUp(), { signal });
    canvas.addEventListener('pointercancel', () => this.#onPointerUp(), { signal });
    canvas.addEventListener('contextmenu', (e) => e.preventDefault(), { signal });
    this.#observer = new ResizeObserver(() => this.#resize());
    this.#observer.observe(scroller);
  }

  // Libera eventos, observador y repintado pendiente. React lo llama al
  // desmontar el componente (en desarrollo, también entre los dos montajes
  // de StrictMode, que de lo contrario duplicarían los eventos).
  destroy(): void {
    this.#abort.abort();
    this.#observer.disconnect();
    cancelAnimationFrame(this.#frame);
  }

  // Muestra un canal. Si las notas no son las que el editor acaba de
  // entregar (cambio de canal, deshacer...), se toma una copia nueva. Con
  // resetScroll se centra la vista en ellas.
  show(view: RollView, resetScroll = false): void {
    this.#view = view;
    if (view.notes !== this.#emitted) {
      this.#notes = structuredClone(view.notes);
      this.#emitted = view.notes;
      this.#selected = null;
      this.#hover = null;
    }
    this.#buildRows();
    this.#updateSize();
    if (resetScroll) this.#centerOnNotes();
    this.draw();
  }

  // Fija la rejilla en ticks; 0 la desactiva (modo libre).
  setSnap(ticks: number): void {
    this.#snap = ticks;
    this.draw();
  }

  // Amplía o reduce la escala horizontal manteniendo fijo el punto x.
  zoom(factor: number, x = KEYS_WIDTH + (this.#scroller.clientWidth - KEYS_WIDTH) / 2): void {
    const tick = this.#tickAt(x);
    this.#zoom = clamp(this.#zoom * factor, MIN_ZOOM, MAX_ZOOM);
    this.#updateSize();
    this.#scroller.scrollLeft = KEYS_WIDTH + tick * this.#zoom - x;
    this.draw();
  }

  // Coloca el cursor de reproducción (null lo oculta). Con follow, la vista
  // avanza para que el cursor no salga de pantalla.
  setPlayhead(tick: number | null, follow = false): void {
    this.#playhead = tick;
    if (follow && tick !== null) {
      const x = this.#x(tick);
      if (x < KEYS_WIDTH || x > this.#scroller.clientWidth - 24) {
        this.#scroller.scrollLeft = tick * this.#zoom - (this.#scroller.clientWidth - KEYS_WIDTH) / 4;
      }
    }
    this.draw();
  }

  // Agrupa los repintados en uno por fotograma.
  draw(): void {
    this.#frame ||= requestAnimationFrame(() => {
      this.#frame = 0;
      this.#paint();
    });
  }

  #emit(): void {
    this.#emitted = structuredClone(this.#notes);
    this.#updateSize();
    this.#callbacks.onChange(this.#emitted);
  }

  // --- Geometría -------------------------------------------------------

  // En percusión la transposición no se aplica: cambiaría el instrumento.
  #offset(): number {
    return this.#view && !this.#view.drums ? this.#view.transpose : 0;
  }

  // Calcula las filas visibles. En percusión, los instrumentos habituales
  // más los que ya use el canal; en el resto, de C1 a C7 ampliado si alguna
  // nota queda fuera.
  #buildRows(): void {
    const offset = this.#offset();
    const pitches = this.#notes.map((n) => n.pitch + offset);
    if (this.#view?.drums) {
      this.#rows = [...new Set([...DRUM_NAMES.keys(), ...pitches])].sort((a, b) => b - a);
    } else {
      const low = pitches.reduce((a, b) => Math.min(a, b), 24);
      const high = pitches.reduce((a, b) => Math.max(a, b), 96);
      this.#rows = Array.from({ length: high - low + 1 }, (_, i) => high - i);
    }
    this.#rowOf = new Map(this.#rows.map((pitch, row) => [pitch, row]));
  }

  #endTick(): number {
    const { length, bars } = this.#view!;
    const last = this.#notes.reduce((end, n) => Math.max(end, n.start + n.duration), length);
    return last + this.#barLength(bars.length - 1) * PADDING_BARS;
  }

  #barLength(index: number): number {
    const { bars, length } = this.#view!;
    if (index < 0) return 4 * PPQ;
    return (index + 1 < bars.length ? bars[index + 1] : length) - bars[index] || 4 * PPQ;
  }

  // Inicios de compás hasta el final editable: los de la canción y, tras
  // ella, compases de la misma duración que el último.
  #barTicks(): number[] {
    const { bars, length } = this.#view!;
    const ticks = bars.length ? [...bars] : [0];
    const step = this.#barLength(bars.length - 1);
    const end = this.#endTick();
    for (let t = Math.max(length, ticks[ticks.length - 1] + step); t <= end; t += step) ticks.push(t);
    return ticks;
  }

  #updateSize(): void {
    this.#spacer.style.width = `${KEYS_WIDTH + this.#endTick() * this.#zoom}px`;
    this.#spacer.style.height = `${RULER_HEIGHT + this.#rows.length * ROW_HEIGHT}px`;
  }

  #resize(): void {
    const ratio = window.devicePixelRatio || 1;
    const { clientWidth, clientHeight } = this.#scroller;
    this.#canvas.width = clientWidth * ratio;
    this.#canvas.height = clientHeight * ratio;
    this.#canvas.style.width = `${clientWidth}px`;
    this.#canvas.style.height = `${clientHeight}px`;
    this.draw();
  }

  #centerOnNotes(): void {
    const offset = this.#offset();
    const notes = this.#notes;
    const pitch = notes.length ? notes.reduce((sum, n) => sum + n.pitch + offset, 0) / notes.length : 60;
    const row = this.#rowOf.get(Math.round(pitch)) ?? this.#rows.length / 2;
    this.#scroller.scrollTop = RULER_HEIGHT + row * ROW_HEIGHT - this.#scroller.clientHeight / 2;
    this.#scroller.scrollLeft = 0;
  }

  #x(tick: number): number {
    return KEYS_WIDTH + tick * this.#zoom - this.#scroller.scrollLeft;
  }

  #y(row: number): number {
    return RULER_HEIGHT + row * ROW_HEIGHT - this.#scroller.scrollTop;
  }

  #tickAt(x: number): number {
    return (x - KEYS_WIDTH + this.#scroller.scrollLeft) / this.#zoom;
  }

  #rowAt(y: number): number {
    return Math.floor((y - RULER_HEIGHT + this.#scroller.scrollTop) / ROW_HEIGHT);
  }

  #point(e: MouseEvent): { x: number; y: number } {
    const rect = this.#canvas.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }

  // Devuelve la nota bajo el punto; se recorren al revés porque la última
  // dibujada queda encima.
  #noteAt(x: number, y: number): Note | null {
    const row = this.#rowAt(y);
    const tick = this.#tickAt(x);
    const offset = this.#offset();
    for (let i = this.#notes.length - 1; i >= 0; i--) {
      const n = this.#notes[i];
      if (this.#rowOf.get(n.pitch + offset) === row && tick >= n.start && tick < n.start + n.duration) return n;
    }
    return null;
  }

  // Indica si x cae en el asa de un extremo de la nota. El asa mide hasta
  // EDGE píxeles, pero en notas cortas se reduce a un tercio de su anchura
  // para que siga habiendo zona central con la que moverla.
  #edgeAt(note: Note, x: number): 'start' | 'end' | null {
    const left = this.#x(note.start);
    const right = this.#x(note.start + note.duration);
    const handle = clamp((right - left) / 3, 2, EDGE);
    if (x >= right - handle) return 'end';
    if (x <= left + handle) return 'start';
    return null;
  }

  // Ajusta un tick a la rejilla, salvo en modo libre o con Alt pulsado.
  #quantize(tick: number, free: boolean, round: (n: number) => number = Math.round): number {
    if (free || this.#snap === 0) return Math.round(tick);
    return round(tick / this.#snap) * this.#snap;
  }

  // Duración mínima de una nota: una celda de la rejilla o, en modo libre, una fusa.
  #minLength(free: boolean): number {
    return free || this.#snap === 0 ? PPQ / 8 : this.#snap;
  }

  // --- Interacción -----------------------------------------------------

  #remove(note: Note): void {
    this.#notes.splice(this.#notes.indexOf(note), 1);
    this.#selected = null;
    this.#hover = null;
    this.#emit();
    this.draw();
  }

  #onPointerDown(e: PointerEvent): void {
    const view = this.#view;
    if (!view) return;
    const { x, y } = this.#point(e);
    const pitch = this.#rows[this.#rowAt(y)];
    if (y < RULER_HEIGHT || pitch === undefined) return;
    if (x < KEYS_WIDTH) {
      this.#callbacks.onPreview(pitch);
      return;
    }
    this.#scroller.focus({ preventScroll: true });
    const hit = this.#noteAt(x, y);

    if (e.button === 2) {
      if (hit) this.#remove(hit);
      return;
    }
    if (e.button !== 0) return;

    let note: Note;
    let mode: Drag['mode'] = 'move';
    if (hit) {
      note = hit;
      const edge = this.#edgeAt(hit, x);
      if (edge) mode = edge === 'end' ? 'resize-end' : 'resize-start';
    } else {
      // Una nota nueva empieza en la rejilla y, si se arrastra, se alarga.
      const start = Math.max(0, this.#quantize(this.#tickAt(x), e.altKey, Math.floor));
      note = { start, duration: this.#noteLength, pitch: pitch - this.#offset(), velocity: DEFAULT_VELOCITY };
      this.#notes.push(note);
      mode = 'resize-end';
    }
    this.#callbacks.onPreview(pitch);
    this.#selected = note;
    this.#drag = {
      mode,
      note,
      created: !hit,
      moved: false,
      startX: x,
      startY: y,
      startTick: this.#tickAt(x),
      origin: { start: note.start, pitch: note.pitch, end: note.start + note.duration },
    };
    this.#canvas.setPointerCapture(e.pointerId);
    this.draw();
  }

  #onPointerMove(e: PointerEvent): void {
    if (!this.#view) return;
    const { x, y } = this.#point(e);
    const drag = this.#drag;
    if (!drag) {
      this.#canvas.style.cursor = this.#cursorAt(x, y);
      const hover = this.#noteAt(x, y);
      if (hover !== this.#hover) {
        this.#hover = hover;
        this.draw();
      }
      return;
    }
    if (!drag.moved && Math.hypot(x - drag.startX, y - drag.startY) < DRAG_THRESHOLD) return;
    drag.moved = true;
    const { note, origin } = drag;
    const free = e.altKey;
    const min = this.#minLength(free);
    const tick = this.#tickAt(x);
    switch (drag.mode) {
      case 'move': {
        note.start = Math.max(0, origin.start + this.#quantize(tick - drag.startTick, free));
        const sounding = this.#rows[clamp(this.#rowAt(y), 0, this.#rows.length - 1)];
        const pitch = sounding - this.#offset();
        if (pitch !== note.pitch && pitch >= 0 && pitch <= 127) {
          note.pitch = pitch;
          this.#callbacks.onPreview(sounding);
        }
        break;
      }
      case 'resize-end':
        note.duration = Math.max(min, this.#quantize(tick, free) - note.start);
        break;
      case 'resize-start':
        // El final queda fijo y se mueve el inicio.
        note.start = clamp(this.#quantize(tick, free), 0, origin.end - min);
        note.duration = origin.end - note.start;
        break;
    }
    this.draw();
  }

  #onPointerUp(): void {
    const drag = this.#drag;
    if (!drag) return;
    this.#drag = null;
    if (drag.moved || drag.created) {
      this.#noteLength = drag.note.duration;
      this.#emit();
    }
    this.draw();
  }

  #cursorAt(x: number, y: number): string {
    if (y < RULER_HEIGHT) return 'default';
    if (x < KEYS_WIDTH) return 'pointer';
    const note = this.#noteAt(x, y);
    if (!note) return 'crosshair';
    return this.#edgeAt(note, x) ? 'ew-resize' : 'grab';
  }

  #onKey(e: KeyboardEvent): void {
    if ((e.key === 'Delete' || e.key === 'Backspace') && this.#selected) {
      e.preventDefault();
      this.#remove(this.#selected);
    }
  }

  // Ctrl + rueda amplía; la rueda sola desplaza como cualquier página.
  #onWheel(e: WheelEvent): void {
    if (!e.ctrlKey || !this.#view) return;
    e.preventDefault();
    this.zoom(e.deltaY < 0 ? 1.25 : 0.8, this.#point(e).x);
  }

  // --- Dibujo ----------------------------------------------------------

  #paint(): void {
    const ctx = this.#ctx;
    const ratio = window.devicePixelRatio || 1;
    const width = this.#canvas.width / ratio;
    const height = this.#canvas.height / ratio;
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    ctx.fillStyle = COLORS.background;
    ctx.fillRect(0, 0, width, height);
    if (!this.#view) return;

    const firstRow = Math.max(0, this.#rowAt(RULER_HEIGHT));
    const lastRow = Math.min(this.#rows.length - 1, this.#rowAt(height));
    const firstTick = Math.max(0, this.#tickAt(KEYS_WIDTH));
    const lastTick = this.#tickAt(width);

    this.#paintRows(firstRow, lastRow, width);
    this.#paintGrid(firstTick, lastTick, width, height);
    this.#paintNotes(firstTick, lastTick, height);
    this.#paintKeys(firstRow, lastRow, height);
    this.#paintRuler(firstTick, lastTick, width);
  }

  #paintRows(firstRow: number, lastRow: number, width: number): void {
    const ctx = this.#ctx;
    const drums = this.#view!.drums;
    for (let row = firstRow; row <= lastRow; row++) {
      const pitch = this.#rows[row];
      const y = this.#y(row);
      const dark = drums ? row % 2 === 1 : BLACK_KEYS.has(pitch % 12);
      ctx.fillStyle = dark ? COLORS.blackRow : COLORS.whiteRow;
      ctx.fillRect(KEYS_WIDTH, y, width, ROW_HEIGHT);
      if (!drums && pitch % 12 === 0) {
        ctx.fillStyle = COLORS.octave;
        ctx.fillRect(KEYS_WIDTH, y + ROW_HEIGHT - 1, width, 1);
      }
    }
  }

  #paintGrid(firstTick: number, lastTick: number, width: number, height: number): void {
    const ctx = this.#ctx;
    const line = (tick: number, color: string) => {
      ctx.fillStyle = color;
      ctx.fillRect(Math.round(this.#x(tick)), RULER_HEIGHT, 1, height);
    };
    if (this.#snap > 0 && this.#snap * this.#zoom >= 6) {
      for (let t = Math.ceil(firstTick / this.#snap) * this.#snap; t <= lastTick; t += this.#snap) line(t, COLORS.subdivision);
    }
    if (PPQ * this.#zoom >= 4) {
      for (let t = Math.ceil(firstTick / PPQ) * PPQ; t <= lastTick; t += PPQ) line(t, COLORS.beat);
    }
    for (const t of this.#barTicks()) if (t >= firstTick && t <= lastTick) line(t, COLORS.bar);

    // Zona posterior al final de la canción: editable, pero sombreada.
    const end = Math.max(KEYS_WIDTH, this.#x(this.#view!.length));
    if (end < width) {
      ctx.fillStyle = COLORS.pastEnd;
      ctx.fillRect(end, RULER_HEIGHT, width - end, height);
    }
  }

  #paintNotes(firstTick: number, lastTick: number, height: number): void {
    const ctx = this.#ctx;
    const { ghosts, color } = this.#view!;
    const offset = this.#offset();
    const rect = (note: Note, pitch: number): [number, number, number, number] | null => {
      const row = this.#rowOf.get(pitch);
      if (row === undefined || note.start > lastTick || note.start + note.duration < firstTick) return null;
      return [this.#x(note.start), this.#y(row) + 1, Math.max(2, note.duration * this.#zoom - 1), ROW_HEIGHT - 2];
    };

    ctx.fillStyle = COLORS.ghost;
    for (const ghost of ghosts) {
      const r = rect(ghost, ghost.pitch);
      if (r) ctx.fillRect(...r);
    }
    for (const note of this.#notes) {
      const r = rect(note, note.pitch + offset);
      if (!r) continue;
      ctx.globalAlpha = 0.45 + 0.55 * (note.velocity / 127);
      ctx.fillStyle = color;
      ctx.fillRect(...r);
      ctx.globalAlpha = 1;
      if (note === this.#selected) {
        ctx.strokeStyle = COLORS.text;
        ctx.lineWidth = 2;
        ctx.strokeRect(r[0] + 1, r[1] + 1, r[2] - 2, r[3] - 2);
      }
      // Asas de los extremos: indican que la nota se puede acortar o alargar.
      if ((note === this.#hover || note === this.#selected) && r[2] >= 8) {
        ctx.fillStyle = COLORS.text;
        ctx.fillRect(r[0] + 2, r[1] + 3, 2, r[3] - 6);
        ctx.fillRect(r[0] + r[2] - 4, r[1] + 3, 2, r[3] - 6);
      }
    }
    if (this.#playhead !== null) {
      ctx.fillStyle = COLORS.playhead;
      ctx.fillRect(Math.round(this.#x(this.#playhead)), RULER_HEIGHT, 2, height);
    }
    this.#paintDuration();
  }

  // Mientras se cambia la duración de una nota, muestra cuánto dura en segundos.
  #paintDuration(): void {
    const drag = this.#drag;
    if (!drag?.moved || drag.mode === 'move') return;
    const { note } = drag;
    const row = this.#rowOf.get(note.pitch + this.#offset());
    if (row === undefined) return;
    const { tempos } = this.#view!;
    const seconds = tickToSeconds(tempos, note.start + note.duration) - tickToSeconds(tempos, note.start);
    const text = `${seconds.toFixed(2).replace('.', ',')} s`;
    const ctx = this.#ctx;
    ctx.font = 'bold 12px system-ui, sans-serif';
    const width = ctx.measureText(text).width + 12;
    const x = this.#x(drag.mode === 'resize-end' ? note.start + note.duration : note.start) + 6;
    const y = Math.max(RULER_HEIGHT + 2, this.#y(row) - 22);
    ctx.fillStyle = COLORS.text;
    ctx.fillRect(x, y, width, 18);
    ctx.fillStyle = COLORS.background;
    ctx.textBaseline = 'middle';
    ctx.fillText(text, x + 6, y + 9);
  }

  #paintKeys(firstRow: number, lastRow: number, height: number): void {
    const ctx = this.#ctx;
    ctx.fillStyle = COLORS.background;
    ctx.fillRect(0, RULER_HEIGHT, KEYS_WIDTH, height);
    ctx.font = '11px system-ui, sans-serif';
    ctx.textBaseline = 'middle';
    for (let row = firstRow; row <= lastRow; row++) {
      const pitch = this.#rows[row];
      const y = this.#y(row);
      if (this.#view!.drums) {
        ctx.fillStyle = COLORS.muted;
        ctx.fillText(DRUM_NAMES.get(pitch) ?? `Perc. ${pitch}`, 6, y + ROW_HEIGHT / 2);
        continue;
      }
      ctx.fillStyle = BLACK_KEYS.has(pitch % 12) ? COLORS.blackKey : COLORS.whiteKey;
      ctx.fillRect(0, y, KEYS_WIDTH - 2, ROW_HEIGHT - 1);
      if (pitch % 12 === 0) {
        ctx.fillStyle = COLORS.background;
        ctx.fillText(noteName(pitch), 6, y + ROW_HEIGHT / 2);
      }
    }
  }

  #paintRuler(firstTick: number, lastTick: number, width: number): void {
    const ctx = this.#ctx;
    ctx.fillStyle = COLORS.blackRow;
    ctx.fillRect(0, 0, width, RULER_HEIGHT);
    ctx.fillStyle = COLORS.muted;
    ctx.font = '11px system-ui, sans-serif';
    ctx.textBaseline = 'middle';
    this.#barTicks().forEach((t, i) => {
      if (t >= firstTick - PPQ * 8 && t <= lastTick) ctx.fillText(String(i + 1), this.#x(t) + 4, RULER_HEIGHT / 2);
    });
    ctx.fillStyle = COLORS.blackRow;
    ctx.fillRect(0, 0, KEYS_WIDTH, RULER_HEIGHT);
  }
}
