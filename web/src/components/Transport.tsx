import { useEffect, useState, type RefObject } from 'react';
import type { PlayMode } from '../hooks/useAudio';
import { formatTime } from '../lib/timing';
import { HealthBar } from './HealthBar';
import { HeartIcon, SpeakerIcon } from './icons';

interface Props {
  player: RefObject<HTMLAudioElement | null>;
  busy: boolean;
  loaded: PlayMode | null; // qué audio está cargado
  outdated: boolean;
  channelName: string;
  onToggle(mode: PlayMode): void;
  onStop(): void;
  onSeek(seconds: number): void;
  onVolume(volume: number): void;
  onToggleSound(): void;
  onWAV(): void;
  onMP3(): void;
  onMIDI(): void;
}

// Estado del reproductor que se muestra en la barra.
function usePlayback(player: RefObject<HTMLAudioElement | null>) {
  const [state, setState] = useState({ time: 0, duration: 0, paused: true, volume: 1, muted: false });

  useEffect(() => {
    const el = player.current;
    if (!el) return;
    const update = () =>
      setState({ time: el.currentTime, duration: Number.isFinite(el.duration) ? el.duration : 0, paused: el.paused, volume: el.volume, muted: el.muted });
    const events = ['timeupdate', 'durationchange', 'play', 'pause', 'emptied', 'volumechange', 'seeked'];
    events.forEach((type) => el.addEventListener(type, update));
    return () => events.forEach((type) => el.removeEventListener(type, update));
  }, [player]);

  return state;
}

// Barra de reproducción flotante en la parte inferior, siempre accesible. El
// progreso y el volumen se muestran como barras de vida. Parar nunca se
// deshabilita: la reproducción debe poder detenerse en todo momento.
export function Transport(props: Props) {
  const { player, busy, loaded, outdated, channelName, onToggle, onStop, onSeek, onVolume, onToggleSound, onWAV, onMP3, onMIDI } = props;
  const { time, duration, paused, volume, muted } = usePlayback(player);
  const playing = (mode: PlayMode) => loaded === mode && !paused;
  const silent = muted || volume === 0;

  return (
    <footer className="fixed inset-x-0 bottom-0 z-40 px-4 pb-4">
      <audio ref={player} preload="auto" />
      {/* Fila superior: reproducción y progreso. Inferior: volumen y descargas. */}
      <div className="mx-auto grid max-w-[1100px] gap-3 border-3 border-line bg-panel px-4 py-3 shadow-pixel">
        <div className="flex flex-wrap items-center gap-3">
          <button type="button" className="btn border-famired bg-famired" disabled={busy} onClick={() => onToggle('song')}>
            {playing('song') ? '❚❚ Pausa' : '▶ Canción'}
          </button>
          <button type="button" className="btn" disabled={busy} title={`Escuchar solo ${channelName}`} onClick={() => onToggle('channel')}>
            {playing('channel') ? '❚❚ Pausa' : '▶ Canal'}
          </button>
          <button type="button" className="btn" onClick={onStop}>
            ■ Parar
          </button>
          <div className="flex min-w-60 flex-1 items-center gap-2.5">
            <span className="text-famired">
              <HeartIcon />
            </span>
            <HealthBar
              label="Posición de la reproducción"
              value={time}
              max={duration}
              step={1}
              format={formatTime}
              onChange={onSeek}
              className="flex-1"
            />
            <span className="font-pixel text-[10px] whitespace-nowrap text-muted">
              {formatTime(time)}/{formatTime(duration)}
            </span>
          </div>
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn px-2 py-1.5 shadow-none"
              aria-label={silent ? 'Activar sonido' : 'Silenciar'}
              title={silent ? 'Activar sonido' : 'Silenciar'}
              onClick={onToggleSound}
            >
              <SpeakerIcon muted={silent} />
            </button>
            <HealthBar
              label="Volumen"
              value={silent ? 0 : volume}
              max={1}
              step={0.01}
              format={(v) => `${Math.round(v * 100)} %`}
              onChange={onVolume}
              className="w-28"
            />
          </div>
          {outdated && <p className="text-[13px] text-gold">Hay cambios: pulsa Canción o Canal para oír la nueva versión.</p>}
          <div className="flex gap-2">
            <button type="button" className="btn" disabled={busy} onClick={onWAV}>
              WAV
            </button>
            <button type="button" className="btn" disabled={busy} onClick={onMP3}>
              MP3
            </button>
            <button type="button" className="btn" disabled={busy} title="En FamiStudio: File → Import → MIDI" onClick={onMIDI}>
              MIDI FamiStudio
            </button>
          </div>
        </div>
      </div>
    </footer>
  );
}
