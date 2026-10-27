import { MAX_TRANSPOSE } from '../lib/project';

interface Props {
  value: number; // semitonos
  onChange(delta: number): void;
}

// Botones para subir o bajar un canal por semitonos y por octavas.
export function Transpose({ value, onChange }: Props) {
  const step = (delta: number, title: string) => (
    <button
      type="button"
      title={title}
      disabled={Math.abs(value + delta) > MAX_TRANSPOSE}
      className="btn px-2 py-1.5 shadow-none"
      onClick={() => onChange(delta)}
    >
      {delta > 0 ? `+${delta}` : delta}
    </button>
  );
  return (
    <div role="group" aria-label="Transposición en semitonos" title="Transposición en semitonos" className="flex items-center gap-1">
      {step(-12, 'Bajar una octava')}
      {step(-1, 'Bajar un semitono')}
      <output className="min-w-[3ch] text-center font-pixel text-[11px]">{value > 0 ? `+${value}` : value}</output>
      {step(1, 'Subir un semitono')}
      {step(12, 'Subir una octava')}
    </div>
  );
}
