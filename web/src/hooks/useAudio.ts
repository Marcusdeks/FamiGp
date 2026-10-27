import { useCallback, useEffect, useRef, useState } from 'react';
import { renderWAV } from '../lib/api';
import type { ProjectPayload } from '../types';

// Qué se escucha: la canción completa o solo el canal abierto en el editor.
export type PlayMode = 'song' | 'channel';

interface Rendered {
  url: string;
  mode: PlayMode;
  payload: ProjectPayload; // estado exacto que se sintetizó
}

// Sintetiza y reproduce el proyecto. El audio solo es válido mientras el
// estado de su modo no cambie: al editar, se detiene y hay que volver a
// sintetizar. Cambiar de canal no afecta a la canción completa, pero sí a la
// escucha del canal.
export function useAudio(payloads: Record<PlayMode, ProjectPayload | null>) {
  const player = useRef<HTMLAudioElement>(null);
  const latest = useRef(payloads);
  const [rendered, setRendered] = useState<Rendered | null>(null);
  const valid = rendered !== null && payloads[rendered.mode] === rendered.payload;

  useEffect(() => {
    latest.current = payloads;
  });

  // Si lo que suena ya no corresponde al proyecto, se para y se descarta.
  useEffect(() => {
    const el = player.current;
    if (valid || !el?.getAttribute('src')) return;
    el.pause();
    el.removeAttribute('src');
    el.load();
  }, [valid]);

  // Libera el WAV anterior al sustituirlo o al desmontar.
  useEffect(() => {
    if (!rendered) return;
    return () => URL.revokeObjectURL(rendered.url);
  }, [rendered]);

  // URL del WAV del modo indicado, sintetizándolo si hace falta.
  const audioURL = useCallback(
    async (mode: PlayMode) => {
      const payload = latest.current[mode];
      if (!payload) throw new Error('No hay ningún proyecto abierto.');
      if (rendered?.mode === mode && rendered.payload === payload) return rendered.url;
      const blob = await renderWAV(payload);
      if (latest.current[mode] !== payload) throw new Error('El proyecto cambió durante la síntesis: vuelve a pulsar Escuchar.');
      const next = { url: URL.createObjectURL(blob), mode, payload };
      setRendered(next);
      return next.url;
    },
    [rendered],
  );

  // Reproduce el modo indicado o, si ya está sonando, lo pausa.
  const toggle = useCallback(
    async (mode: PlayMode) => {
      const el = player.current!;
      if (valid && rendered?.mode === mode && !el.paused) {
        el.pause();
        return;
      }
      const url = await audioURL(mode);
      if (el.getAttribute('src') !== url) el.src = url;
      // Si la síntesis tarda, el navegador puede bloquear la reproducción
      // automática; basta con volver a pulsar.
      await el.play().catch(() => {});
    },
    [audioURL, rendered, valid],
  );

  const stop = useCallback(() => {
    const el = player.current;
    if (!el) return;
    el.pause();
    el.currentTime = 0;
  }, []);

  const seek = useCallback((seconds: number) => {
    const el = player.current;
    if (el && Number.isFinite(el.duration)) el.currentTime = seconds;
  }, []);

  const setVolume = useCallback((volume: number) => {
    const el = player.current;
    if (!el) return;
    el.volume = volume;
    el.muted = volume === 0;
  }, []);

  // Alterna el silencio; si el volumen estaba a cero, lo restablece al máximo.
  const toggleSound = useCallback(() => {
    const el = player.current;
    if (!el) return;
    if (el.muted || el.volume === 0) setVolume(el.volume || 1);
    else el.muted = true;
  }, [setVolume]);

  return {
    player,
    toggle,
    stop,
    seek,
    setVolume,
    toggleSound,
    audioURL,
    loaded: rendered && valid ? rendered.mode : null, // modo del audio cargado y vigente
    outdated: rendered !== null && !valid,
  };
}
