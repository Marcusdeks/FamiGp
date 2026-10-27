import { useState, type KeyboardEvent, type PointerEvent } from 'react';

interface Props {
  label: string; // nombre accesible del control
  value: number;
  max: number;
  step: number; // incremento con las flechas del teclado y precisión al hacer clic
  format(value: number): string; // texto de la etiqueta flotante y del lector de pantalla
  onChange(value: number): void;
  className?: string;
}

// Deslizador con forma de barra de vida: un relleno rojo continuo dentro de
// un marco pixelado. Al pasar el ratón muestra el valor exacto que se
// elegirá; se maneja con clic, arrastre o teclado (role="slider").
export function HealthBar({ label, value, max, step, format, onChange, className = '' }: Props) {
  const [hover, setHover] = useState<{ x: number; value: number } | null>(null);
  const fraction = max > 0 ? Math.min(1, value / max) : 0;
  const clamp = (v: number) => Math.min(max, Math.max(0, v));

  // Valor bajo el puntero, redondeado a la precisión del control.
  const valueAt = (e: PointerEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect();
    const x = Math.min(rect.width, Math.max(0, e.clientX - rect.left));
    return { x, value: clamp(Math.round(((x / rect.width) * max) / step) * step) };
  };

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const keys: Record<string, number> = { ArrowRight: value + step, ArrowUp: value + step, ArrowLeft: value - step, ArrowDown: value - step, Home: 0, End: max };
    if (!(e.key in keys)) return;
    e.preventDefault();
    onChange(clamp(keys[e.key]));
  };

  return (
    <div
      role="slider"
      tabIndex={0}
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={max}
      aria-valuenow={value}
      aria-valuetext={format(value)}
      className={`relative h-7 cursor-pointer touch-none border-3 border-line bg-night p-[3px] focus-visible:outline-2 focus-visible:outline-cyan ${className}`}
      onPointerDown={(e) => {
        e.currentTarget.setPointerCapture(e.pointerId);
        onChange(valueAt(e).value);
      }}
      onPointerMove={(e) => {
        const point = valueAt(e);
        setHover(point);
        if (e.currentTarget.hasPointerCapture(e.pointerId)) onChange(point.value);
      }}
      onPointerLeave={() => setHover(null)}
      onKeyDown={onKeyDown}
    >
      <div className="h-full bg-famired" style={{ width: `${fraction * 100}%` }} />
      {hover && max > 0 && (
        <span
          className="pointer-events-none absolute bottom-full mb-1.5 -translate-x-1/2 bg-ink px-1.5 py-0.5 font-pixel text-[9px] whitespace-nowrap text-night"
          style={{ left: hover.x }}
        >
          {format(hover.value)}
        </span>
      )}
    </div>
  );
}
