import { useEffect, useRef, useState, type RefObject } from 'react';
import { PianoRoll, type RollView } from '../lib/pianoRoll';
import { secondsToTick } from '../lib/timing';
import { PPQ, type Note, type Tempo } from '../types';
import { Panel } from './Panel';
import { Transpose } from './Transpose';

interface Props {
  view: RollView;
  channelName: string;
  resetKey: string; // cambia al abrir otro canal o proyecto para recentrar la vista
  tempos: Tempo[];
  player: RefObject<HTMLAudioElement | null>;
  canUndo: boolean;
  canRedo: boolean;
  onChange(notes: Note[]): void;
  onPreview(pitch: number): void;
  onTranspose(delta: number): void;
  onUndo(): void;
  onRedo(): void;
}

const SNAPS = [
  { label: 'Negra', ticks: PPQ },
  { label: 'Corchea', ticks: PPQ / 2 },
  { label: 'Semicorchea', ticks: PPQ / 4 },
  { label: 'Fusa', ticks: PPQ / 8 },
  { label: 'Tresillo de corchea', ticks: PPQ / 3 },
  { label: 'Libre', ticks: 0 },
];

// Envoltorio de React para el piano roll: crea la instancia imperativa una
// sola vez y le pasa los datos cuando cambian.
export function EditorPanel(props: Props) {
  const { view, channelName, resetKey, tempos, player, canUndo, canRedo, onTranspose, onUndo, onRedo } = props;
  const scroller = useRef<HTMLDivElement>(null);
  const spacer = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);
  const roll = useRef<PianoRoll | null>(null);
  const shownKey = useRef<string | null>(null);
  const callbacks = useRef({ onChange: props.onChange, onPreview: props.onPreview });
  const [snap, setSnap] = useState(PPQ / 4);

  // El editor llama siempre a la versión más reciente de los callbacks.
  useEffect(() => {
    callbacks.current = { onChange: props.onChange, onPreview: props.onPreview };
  });

  useEffect(() => {
    const instance = new PianoRoll(scroller.current!, spacer.current!, canvas.current!, {
      onChange: (notes) => callbacks.current.onChange(notes),
      onPreview: (pitch) => callbacks.current.onPreview(pitch),
    });
    roll.current = instance;
    shownKey.current = null;
    return () => {
      instance.destroy();
      roll.current = null;
    };
  }, []);

  useEffect(() => {
    roll.current?.show(view, shownKey.current !== resetKey);
    shownKey.current = resetKey;
  }, [view, resetKey]);

  useEffect(() => roll.current?.setSnap(snap), [snap]);

  // Cursor de reproducción: avanza con el audio y se borra al descartarlo.
  useEffect(() => {
    const el = player.current;
    if (!el) return;
    let frame = 0;
    const draw = (follow: boolean) => roll.current?.setPlayhead(secondsToTick(tempos, el.currentTime), follow);
    const loop = () => {
      draw(true);
      if (!el.paused) frame = requestAnimationFrame(loop);
    };
    const onPlay = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(loop);
    };
    const onSeeked = () => draw(false);
    const onEmptied = () => {
      cancelAnimationFrame(frame);
      roll.current?.setPlayhead(null);
    };
    el.addEventListener('play', onPlay);
    el.addEventListener('seeked', onSeeked);
    el.addEventListener('emptied', onEmptied);
    return () => {
      cancelAnimationFrame(frame);
      el.removeEventListener('play', onPlay);
      el.removeEventListener('seeked', onSeeked);
      el.removeEventListener('emptied', onEmptied);
    };
  }, [player, tempos]);

  return (
    <Panel step={4} title="Editor" badge={channelName}>
      <div className="mb-3 flex flex-wrap items-center gap-4 text-sm">
        <label className="flex items-center gap-2" title="Con Alt pulsado se ignora la rejilla al arrastrar">
          Rejilla
          <select value={snap} onChange={(e) => setSnap(Number(e.target.value))}>
            {SNAPS.map(({ label, ticks }) => (
              <option key={ticks} value={ticks}>
                {label}
              </option>
            ))}
          </select>
        </label>
        <div className="flex gap-1.5">
          <button type="button" className="btn" title="Alejar (Ctrl + rueda)" onClick={() => roll.current?.zoom(0.8)}>
            -
          </button>
          <button type="button" className="btn" title="Acercar (Ctrl + rueda)" onClick={() => roll.current?.zoom(1.25)}>
            +
          </button>
        </div>
        <div className="flex gap-1.5">
          <button type="button" className="btn" title="Deshacer (Ctrl+Z)" disabled={!canUndo} onClick={onUndo}>
            Deshacer
          </button>
          <button type="button" className="btn" title="Rehacer (Ctrl+Y)" disabled={!canRedo} onClick={onRedo}>
            Rehacer
          </button>
        </div>
        {view.drums ? (
          <span className="text-[13px] text-muted">Sin transposición: cambiaría el instrumento</span>
        ) : (
          <Transpose value={view.transpose} onChange={onTranspose} />
        )}
      </div>

      {/* El espaciador da al contenedor el tamaño de la canción; el canvas,
          fijo con sticky, solo dibuja la zona visible. */}
      <div
        ref={scroller}
        tabIndex={0}
        aria-label="Editor de notas"
        className="relative h-[min(480px,60vh)] overflow-auto overscroll-contain border-3 border-line focus-visible:outline-2 focus-visible:outline-cyan"
      >
        <div ref={spacer} className="absolute top-0 left-0" />
        <canvas ref={canvas} className="sticky top-0 left-0 block" />
      </div>
      <p className="mt-3 text-sm text-muted">
        Clic: añadir nota (arrastra para alargarla) · Arrastrar: mover · Bordes de una nota: acortar o alargar (Alt: sin rejilla)
        · Clic derecho o Supr: borrar · Clic en el teclado: escuchar · Ctrl + rueda: zoom
      </p>
    </Panel>
  );
}
