import { useEffect, useMemo, useRef, useState } from 'react';
import { ChannelList } from './components/ChannelList';
import { EditorPanel } from './components/EditorPanel';
import { PresetPicker } from './components/PresetPicker';
import { ProjectList } from './components/ProjectList';
import { Timeline } from './components/Timeline';
import { Transport } from './components/Transport';
import { useAudio, type PlayMode } from './hooks/useAudio';
import { useProjects } from './hooks/useProjects';
import { useTask } from './hooks/useTask';
import { exportMIDI, getPresets } from './lib/api';
import { cutProject } from './lib/cut';
import type { RollView } from './lib/pianoRoll';
import { previewNote } from './lib/preview';
import { CHANNEL_COLORS, newLine, replaceLine, toPayload } from './lib/project';
import type { Line, Note, Preset, Project } from './types';

function download(url: string, name: string) {
  Object.assign(document.createElement('a'), { href: url, download: name }).click();
}

export function App() {
  const [presets, setPresets] = useState<Preset[]>([]);
  const [presetId, setPresetId] = useState<string | null>(null); // chip elegido sin proyecto abierto
  const { status, busy, run } = useTask();
  const projects = useProjects();
  const { active, channel } = projects;
  const preset = presets.find((p) => p.id === (active?.preset ?? presetId)) ?? presets[0];

  useEffect(() => {
    run('Cargando chips…', async () => setPresets(await getPresets()));
  }, [run]);

  // Lo que puede sonar: la canción completa (sin los canales silenciados) o
  // solo el canal abierto en el editor.
  const songPayload = useMemo(() => (active && preset ? toPayload(active, preset) : null), [active, preset]);
  const channelPayload = useMemo(() => (active && preset ? toPayload(active, preset, channel) : null), [active, preset, channel]);
  const audio = useAudio({ song: songPayload, channel: channelPayload });

  // Datos del canal abierto en el editor. Las notas del resto de canales
  // tonales se muestran atenuadas, a su altura audible, como guía.
  const view = useMemo<RollView | null>(() => {
    const voice = preset?.voices[channel];
    const line = active?.lines[channel];
    if (!active || !preset || !voice || !line) return null;
    const ghosts = voice.drums
      ? []
      : active.lines.flatMap((other, i) =>
          i === channel || preset.voices[i]?.drums ? [] : other.notes.map((n) => ({ ...n, pitch: n.pitch + other.transpose })),
        );
    return {
      notes: line.notes,
      transpose: line.transpose,
      drums: voice.drums,
      color: CHANNEL_COLORS[channel % CHANNEL_COLORS.length],
      ghosts,
      bars: active.song.bars,
      length: active.song.length,
      tempos: active.song.tempos,
    };
  }, [active, preset, channel]);

  // Atajos de deshacer y rehacer. Se lee la versión más reciente mediante una
  // referencia para no volver a registrar el evento en cada renderizado.
  const shortcuts = useRef({ undo: projects.undo, redo: projects.redo });
  useEffect(() => {
    shortcuts.current = { undo: projects.undo, redo: projects.redo };
  });
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      const key = e.key.toLowerCase();
      if (key === 'z' && !e.shiftKey) shortcuts.current.undo();
      else if (key === 'y' || (key === 'z' && e.shiftKey)) shortcuts.current.redo();
      else return;
      e.preventDefault();
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, []);

  const fileName = (extension: string) => `${active?.name} (${preset?.name}).${extension}`;

  function addFiles(files: File[]) {
    if (preset) run('Leyendo partituras…', () => projects.add(files, preset));
  }

  function addEmpty() {
    if (preset) run('Creando proyecto…', () => projects.addEmpty(preset, presets));
  }

  function removeProject(project: Project) {
    if (confirm(`¿Eliminar el proyecto «${project.name}»? No se puede deshacer.`)) run('Eliminando…', () => projects.remove(project));
  }

  function selectPreset(next: Preset) {
    setPresetId(next.id);
    projects.setPreset(next);
  }

  // Aplica un cambio a una línea; queda registrado para deshacerlo.
  const editLine = (index: number, change: (line: Line) => Line) =>
    projects.editLines((lines) => replaceLine(lines, index, change(lines[index])), index);

  const editNotes = (notes: Note[]) => editLine(channel, (line) => ({ ...line, notes }));
  const setSource = (index: number, track: number) =>
    active && editLine(index, (line) => ({ ...newLine(active.song, track), transpose: line.transpose, muted: line.muted }));
  const transpose = (delta: number) => editLine(channel, (line) => ({ ...line, transpose: line.transpose + delta }));
  const toggleMute = (index: number) => editLine(index, (line) => ({ ...line, muted: !line.muted }));

  // Oye una nota con la voz del canal abierto, al volumen del reproductor.
  const preview = (pitch: number) => {
    const el = audio.player.current;
    if (preset) previewNote(preset.id, channel, pitch, el?.muted ? 0 : (el?.volume ?? 1));
  };

  const togglePlayback = (mode: PlayMode) => run('Sintetizando…', () => audio.toggle(mode));

  return (
    <>
      <header className="px-4 pt-10 pb-6 text-center">
        <h1 className="font-pixel text-[28px] text-famired [text-shadow:4px_4px_0_#000] sm:text-[40px]">
          FAMI<span className="text-gold">GP</span>
        </h1>
        <p className="mt-4 text-muted">Convierte tus partituras de Guitar Pro en música de 8 y 16 bits.</p>
      </header>

      <main className="mx-auto grid max-w-[1100px] gap-5 px-4 pb-44">
        <ProjectList projects={projects.projects} active={active} onOpen={projects.open} onRemove={removeProject} onFiles={addFiles} onEmpty={addEmpty} />
        <PresetPicker presets={presets} selected={preset} onSelect={selectPreset} />

        {active && preset && view && (
          <>
            <ChannelList
              project={active}
              preset={preset}
              channel={channel}
              onSelect={projects.setChannel}
              onSource={setSource}
              onToggleMute={toggleMute}
            />
            <EditorPanel
              view={view}
              channelName={preset.voices[channel].name}
              resetKey={`${active.id}:${channel}`}
              tempos={active.song.tempos}
              player={audio.player}
              canUndo={projects.canUndo}
              canRedo={projects.canRedo}
              onChange={editNotes}
              onPreview={preview}
              onTranspose={transpose}
              onUndo={projects.undo}
              onRedo={projects.redo}
            />
            <Timeline
              key={active.id}
              project={active}
              preset={preset}
              player={audio.player}
              onSeek={audio.seek}
              onCut={(from, to) => projects.editProject((project) => cutProject(project, from, to))}
            />
          </>
        )}

        {!projects.persistent && (
          <p className="text-center text-sm text-famired">Este navegador no permite guardar: los proyectos se perderán al cerrar la página.</p>
        )}
        <p role="status" aria-live="polite" className={`min-h-6 text-center font-pixel text-[10px] leading-relaxed ${status?.error ? 'text-famired' : 'text-gold'}`}>
          {status?.text}
        </p>
      </main>

      {active && preset && songPayload && (
        <Transport
          player={audio.player}
          busy={busy}
          loaded={audio.loaded}
          outdated={audio.outdated}
          channelName={preset.voices[channel]?.name ?? ''}
          onToggle={togglePlayback}
          onStop={audio.stop}
          onSeek={audio.seek}
          onVolume={audio.setVolume}
          onToggleSound={audio.toggleSound}
          onWAV={() => run('Sintetizando…', async () => download(await audio.audioURL('song'), fileName('wav')))}
          onMIDI={() =>
            run('Generando MIDI…', async () => {
              const url = URL.createObjectURL(await exportMIDI(songPayload));
              download(url, fileName('mid'));
              // Margen para que el navegador empiece la descarga antes de liberar la URL.
              setTimeout(() => URL.revokeObjectURL(url), 10_000);
            })
          }
        />
      )}
    </>
  );
}
