import { useEffect, useMemo, useRef, useState, type PointerEvent, type RefObject } from 'react';
import { CHANNEL_COLORS } from '../lib/project';
import { formatPrecise, formatTime, secondsToTick, tickToSeconds } from '../lib/timing';
import { PPQ, type Preset, type Project } from '../types';
import { Panel } from './Panel';

interface Props {
  project: Project;
  preset: Preset;
  player: RefObject<HTMLAudioElement | null>;
  onSeek(seconds: number): void;
  onCut(from: number, to: number): void; // tramo en ticks
}

const RULER_HEIGHT = 20;
const LANE_HEIGHT = 24;
const DRAG_THRESHOLD = 3; // píxeles para distinguir un clic de una selección
const TIME_STEPS = [1, 2, 5, 10, 15, 30, 60, 120, 300]; // separaciones posibles de la regla, en segundos

interface Selection {
  from: number; // ticks, siempre from < to
  to: number;
}

interface Scene {
  project: Project;
  preset: Preset;
  end: number; // tick final de la vista
  selection: Selection | null;
  width: number;
  height: number;
}

// Dibuja la línea de tiempo completa. Es una función pura de la escena más
// la posición del audio (null si no hay audio cargado).
function drawTimeline(canvas: HTMLCanvasElement, { project, preset, end, selection, width, height }: Scene, playhead: number | null) {
  const ctx = canvas.getContext('2d');
  if (!ctx || width === 0) return;
  const { tempos } = project.song;
  const ratio = window.devicePixelRatio || 1;
  ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
  const x = (tick: number) => (tick / end) * width;

  ctx.fillStyle = '#0e0b1f';
  ctx.fillRect(0, 0, width, height);

  // Regla de tiempo con la separación más corta que deje sitio a las etiquetas.
  const total = tickToSeconds(tempos, end);
  const step = TIME_STEPS.find((s) => (s / total) * width >= 56) ?? TIME_STEPS[TIME_STEPS.length - 1];
  ctx.font = '10px system-ui, sans-serif';
  ctx.textBaseline = 'middle';
  for (let s = 0; s <= total; s += step) {
    const px = Math.round(x(secondsToTick(tempos, s)));
    ctx.fillStyle = '#3a3370';
    ctx.fillRect(px, RULER_HEIGHT - 6, 1, height);
    ctx.fillStyle = '#a9a2d6';
    ctx.fillText(formatTime(s), px + 3, RULER_HEIGHT / 2 - 1);
  }

  // Franjas de los canales: cada nota es un trazo a su altura relativa.
  preset.voices.forEach((voice, i) => {
    const line = project.lines[i];
    const top = RULER_HEIGHT + i * LANE_HEIGHT;
    ctx.fillStyle = i % 2 ? '#141033' : '#1b1740';
    ctx.fillRect(0, top, width, LANE_HEIGHT);
    if (line?.notes.length) {
      const low = line.notes.reduce((m, n) => Math.min(m, n.pitch), 127);
      const span = Math.max(1, line.notes.reduce((m, n) => Math.max(m, n.pitch), 0) - low);
      ctx.globalAlpha = line.muted ? 0.25 : 0.9;
      ctx.fillStyle = CHANNEL_COLORS[i % CHANNEL_COLORS.length];
      for (const n of line.notes) {
        const left = x(n.start);
        ctx.fillRect(left, top + 3 + (1 - (n.pitch - low) / span) * (LANE_HEIGHT - 8), Math.max(1, x(n.start + n.duration) - left), 2);
      }
      ctx.globalAlpha = 1;
    }
    ctx.fillStyle = '#a9a2d6';
    ctx.fillText(voice.name, 4, top + LANE_HEIGHT / 2);
  });

  if (selection) {
    const a = x(selection.from);
    const b = x(selection.to);
    ctx.fillStyle = 'rgba(60, 188, 252, 0.25)';
    ctx.fillRect(a, 0, b - a, height);
    ctx.fillStyle = '#3cbcfc';
    ctx.fillRect(a - 1, 0, 2, height);
    ctx.fillRect(b - 1, 0, 2, height);
  }

  if (playhead !== null) {
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(Math.round(x(secondsToTick(tempos, playhead))), 0, 2, height);
  }
}

// Vista de la canción completa, como la de un reproductor: una franja por
// canal con sus notas en miniatura y una regla en segundos. Un clic salta a
// ese punto; arrastrando se selecciona un tramo para cortarlo.
export function Timeline({ project, preset, player, onSeek, onCut }: Props) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const drag = useRef<{ startX: number; anchor: number; moved: boolean } | null>(null);
  const [selection, setSelection] = useState<Selection | null>(null);
  const [width, setWidth] = useState(0);
  const { tempos } = project.song;
  const height = RULER_HEIGHT + preset.voices.length * LANE_HEIGHT;

  // Final de la vista: la canción o la última nota, si se escribió más allá.
  const end = useMemo(
    () => project.lines.reduce((last, line) => line.notes.reduce((m, n) => Math.max(m, n.start + n.duration), last), project.song.length) || 4 * PPQ,
    [project],
  );

  // Escena más reciente, para los eventos y bucles de animación.
  const scene = useRef<Scene>({ project, preset, end, selection, width, height });
  useEffect(() => {
    scene.current = { project, preset, end, selection, width, height };
  });

  // Repinta con la escena actual y la posición del audio, si lo hay.
  const repaint = useRef(() => {
    const el = canvas.current;
    const audio = player.current;
    if (el) drawTimeline(el, scene.current, audio?.getAttribute('src') ? audio.currentTime : null);
  });

  useEffect(() => repaint.current());

  // Ajusta la resolución del canvas a su tamaño en pantalla (cambiarla borra
  // el canvas, así que se repinta al momento).
  useEffect(() => {
    const el = canvas.current!;
    const observer = new ResizeObserver(() => {
      const ratio = window.devicePixelRatio || 1;
      el.width = el.clientWidth * ratio;
      el.height = height * ratio;
      scene.current = { ...scene.current, width: el.clientWidth };
      setWidth(el.clientWidth);
      repaint.current();
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, [height]);

  // Mientras suena el audio, el cursor avanza en cada fotograma.
  useEffect(() => {
    const audio = player.current;
    if (!audio) return;
    let frame = 0;
    const loop = () => {
      repaint.current();
      if (!audio.paused) frame = requestAnimationFrame(loop);
    };
    const start = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(loop);
    };
    const once = () => repaint.current();
    const events: [string, () => void][] = [['play', start], ['seeked', once], ['emptied', once], ['pause', once]];
    events.forEach(([type, handler]) => audio.addEventListener(type, handler));
    return () => {
      cancelAnimationFrame(frame);
      events.forEach(([type, handler]) => audio.removeEventListener(type, handler));
    };
  }, [player]);

  // Tick bajo el puntero; la selección se ajusta a pulsos salvo con Alt.
  const tickAt = (e: PointerEvent<HTMLCanvasElement>, free = true) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const tick = (Math.min(rect.width, Math.max(0, e.clientX - rect.left)) / rect.width) * end;
    return free ? Math.round(tick) : Math.min(end, Math.round(tick / PPQ) * PPQ);
  };

  const cut = () => {
    if (!selection) return;
    onCut(selection.from, selection.to);
    setSelection(null);
  };

  const duration = selection ? tickToSeconds(tempos, selection.to) - tickToSeconds(tempos, selection.from) : 0;

  return (
    <Panel step={5} title="Canción" badge={formatTime(tickToSeconds(tempos, end))}>
      <p className="mb-3 text-sm text-muted">
        Clic: saltar a ese punto · Arrastrar: seleccionar un tramo (se ajusta a los pulsos; con Alt, libre) · Supr o Cortar: eliminarlo
        y unir lo que queda.
      </p>
      <canvas
        ref={canvas}
        tabIndex={0}
        aria-label="Línea de tiempo de la canción"
        className="block w-full cursor-text touch-none border-3 border-line focus-visible:outline-2 focus-visible:outline-cyan"
        style={{ height }}
        onPointerDown={(e) => {
          e.currentTarget.setPointerCapture(e.pointerId);
          drag.current = { startX: e.clientX, anchor: tickAt(e, e.altKey), moved: false };
        }}
        onPointerMove={(e) => {
          const d = drag.current;
          if (!d || (!d.moved && Math.abs(e.clientX - d.startX) < DRAG_THRESHOLD)) return;
          d.moved = true;
          const tick = tickAt(e, e.altKey);
          setSelection(tick === d.anchor ? null : { from: Math.min(tick, d.anchor), to: Math.max(tick, d.anchor) });
        }}
        onPointerUp={(e) => {
          const d = drag.current;
          drag.current = null;
          if (d && !d.moved) {
            setSelection(null);
            onSeek(tickToSeconds(tempos, tickAt(e)));
          }
        }}
        onKeyDown={(e) => {
          if (e.key === 'Delete' || e.key === 'Backspace') {
            e.preventDefault();
            cut();
          } else if (e.key === 'Escape') setSelection(null);
        }}
      />
      <div className="mt-3 flex flex-wrap items-center gap-3 text-sm">
        <span className="min-w-60 flex-1 text-muted">
          {selection
            ? `Selección: ${formatPrecise(tickToSeconds(tempos, selection.from))} – ${formatPrecise(tickToSeconds(tempos, selection.to))} (${duration.toFixed(1).replace('.', ',')} s)`
            : 'Sin tramo seleccionado'}
        </span>
        <button type="button" className="btn" disabled={!selection} onClick={() => setSelection(null)}>
          Quitar selección
        </button>
        <button type="button" className="btn border-famired" disabled={!selection} onClick={cut}>
          ✂ Cortar selección
        </button>
      </div>
    </Panel>
  );
}
