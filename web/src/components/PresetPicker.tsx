import type { Preset } from '../types';
import { Panel } from './Panel';

interface Props {
  presets: Preset[];
  selected: Preset | undefined;
  onSelect(preset: Preset): void;
}

// Menú de chips de sonido, como un grupo de botones de opción accesible.
export function PresetPicker({ presets, selected, onSelect }: Props) {
  return (
    <Panel step={2} title="Sonido">
      <div role="radiogroup" aria-label="Chip de sonido" className="grid grid-cols-[repeat(auto-fill,minmax(190px,1fr))] gap-3">
        {presets.map((preset) => (
          <label
            key={preset.id}
            className={`relative grid cursor-pointer gap-2 border-3 p-3.5 has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-cyan ${
              preset === selected ? 'border-gold bg-gold/10' : 'border-line'
            }`}
          >
            <input type="radio" name="preset" className="absolute opacity-0" checked={preset === selected} onChange={() => onSelect(preset)} />
            <span className="font-pixel text-[11px] leading-snug">{preset.name}</span>
            <span className="text-[13px] text-muted">{preset.description}</span>
          </label>
        ))}
      </div>
    </Panel>
  );
}
