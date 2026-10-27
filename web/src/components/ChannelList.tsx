import { CHANNEL_COLORS } from '../lib/project';
import type { Preset, Project } from '../types';
import { SpeakerIcon } from './icons';
import { Panel } from './Panel';

interface Props {
  project: Project;
  preset: Preset;
  channel: number;
  onSelect(channel: number): void;
  onSource(channel: number, track: number): void;
  onToggleMute(channel: number): void;
}

// Una fila por canal del chip: pista de la que copiar las notas, botón de
// silencio y acceso al editor.
export function ChannelList({ project, preset, channel, onSelect, onSource, onToggleMute }: Props) {
  return (
    <Panel step={3} title="Canales">
      <p className="text-sm text-muted">
        Cada canal toca una línea. Elige de qué pista copiar sus notas, silénciala con el altavoz y ábrela en el editor para
        modificarla o escribirla desde cero.
      </p>
      <div className="mt-4 grid gap-2.5">
        {preset.voices.map((voice, i) => {
          const line = project.lines[i];
          const muted = Boolean(line.muted);
          return (
            <div key={voice.name} className="grid grid-cols-[1fr_auto_auto] items-center gap-3 md:grid-cols-[130px_minmax(0,1fr)_auto_90px_auto]">
              <span
                className={`border-l-6 pl-2.5 font-pixel text-[10px] ${muted ? 'opacity-50' : ''}`}
                style={{ borderColor: CHANNEL_COLORS[i % CHANNEL_COLORS.length] }}
              >
                {voice.name}
              </span>
              <select
                aria-label={`Pista de ${voice.name}`}
                title="Sustituye las notas del canal por las de una pista"
                className="col-span-3 md:col-span-1"
                value={line.source}
                onChange={(e) => onSource(i, Number(e.target.value))}
              >
                <option value={-1}>Vacío</option>
                {project.song.tracks.map((track, index) => (
                  <option key={index} value={index}>
                    {`${index + 1}. ${track.name || 'Sin nombre'}${track.percussion ? ' (batería)' : ''}`}
                  </option>
                ))}
              </select>
              <button
                type="button"
                aria-pressed={muted}
                aria-label={`${muted ? 'Activar' : 'Silenciar'} ${voice.name}`}
                title={muted ? 'Canal silenciado: pulsa para que vuelva a sonar' : 'Silenciar canal'}
                className={`btn px-2.5 py-1.5 ${muted ? 'border-famired text-famired' : ''}`}
                onClick={() => onToggleMute(i)}
              >
                <SpeakerIcon muted={muted} />
              </button>
              <span className="text-[13px] text-muted">
                {line.notes.length} {line.notes.length === 1 ? 'nota' : 'notas'}
              </span>
              <button type="button" className={`btn ${i === channel ? 'border-gold text-gold' : ''}`} onClick={() => onSelect(i)}>
                Editar
              </button>
            </div>
          );
        })}
      </div>
    </Panel>
  );
}
