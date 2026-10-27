import { useCallback, useEffect, useState } from 'react';
import { parseSong } from '../lib/api';
import { createProject, emptyHistory, emptySong, fitLines, MAX_PROJECTS, record, redo, undo, type History } from '../lib/project';
import * as storage from '../lib/storage';
import type { Line, Preset, Project } from '../types';

// Estado de los proyectos: lista guardada, proyecto y canal activos, e
// historial para deshacer. Cada cambio se guarda al momento en IndexedDB;
// si el navegador no lo permite, se sigue trabajando en memoria.
export function useProjects() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [channel, setChannel] = useState(0);
  const [history, setHistory] = useState<History>(emptyHistory);
  const [persistent, setPersistent] = useState(true);

  const active = projects.find((p) => p.id === activeId) ?? null;

  const open = useCallback((project: Project) => {
    setActiveId(project.id);
    setChannel(Math.max(0, project.lines.findIndex((line) => line.notes.length > 0)));
    setHistory(emptyHistory);
  }, []);

  useEffect(() => {
    storage
      .loadProjects()
      .then((saved) => {
        const sorted = saved.sort((a, b) => b.created - a.created);
        setProjects(sorted);
        if (sorted.length) open(sorted[0]);
      })
      .catch(() => setPersistent(false));
  }, [open]);

  // Sustituye un proyecto en la lista y lo guarda.
  const commit = useCallback(
    (next: Project) => {
      setProjects((list) => list.map((p) => (p.id === next.id ? next : p)));
      if (persistent) storage.saveProject(next).catch(() => setPersistent(false));
    },
    [persistent],
  );

  const room = () => {
    const free = MAX_PROJECTS - projects.length;
    if (free <= 0) throw new Error(`Ya tienes ${MAX_PROJECTS} proyectos: elimina uno para añadir otro.`);
    return free;
  };

  // Añade proyectos nuevos al principio de la lista, los guarda y abre el primero.
  async function insert(created: Project[]) {
    setProjects((list) => [...[...created].reverse(), ...list]);
    if (persistent) await Promise.all(created.map((p) => storage.saveProject(p))).catch(() => setPersistent(false));
    open(created[0]);
  }

  // Crea un proyecto por cada archivo de Guitar Pro hasta llenar el límite.
  // Los archivos que no caben no se llegan a enviar. Si alguno no se añade,
  // se lanza un error que lo explica, pero los demás se conservan.
  async function add(files: File[], preset: Preset) {
    const free = room();
    const created: Project[] = [];
    const problems: string[] = [];
    for (const file of files.slice(0, free)) {
      try {
        created.push(createProject(await parseSong(file), preset));
      } catch (err) {
        problems.push(`${file.name}: ${err instanceof Error ? err.message : err}.`);
      }
    }
    if (created.length) await insert(created);
    const skipped = files.length - Math.min(files.length, free);
    if (skipped) problems.push(`${skipped} archivo(s) sin añadir: el máximo es ${MAX_PROJECTS} proyectos.`);
    if (problems.length) throw new Error(problems.join(' '));
  }

  // Crea un proyecto vacío para componer desde cero.
  async function addEmpty(preset: Preset, presets: Preset[]) {
    room();
    const count = projects.filter((p) => p.song.tracks.length === 0).length + 1;
    await insert([createProject(emptySong(presets, `Proyecto vacío ${count}`), preset)]);
  }

  async function remove(project: Project) {
    const rest = projects.filter((p) => p !== project);
    setProjects(rest);
    if (persistent) await storage.deleteProject(project.id).catch(() => setPersistent(false));
    if (project.id === activeId) {
      if (rest.length) open(rest[0]);
      else setActiveId(null);
    }
  }

  // Aplica un cambio deshacible al proyecto; `edited` es el canal al que se
  // vuelve al deshacer.
  function editProject(change: (project: Project) => Project, edited = channel) {
    if (!active) return;
    setHistory((h) => record(h, { lines: active.lines, song: active.song, channel: edited }));
    commit({ ...change(active), updated: Date.now() });
  }

  function editLines(change: (lines: Line[]) => Line[], edited: number) {
    editProject((project) => ({ ...project, lines: change(project.lines) }), edited);
  }

  function travel(step: typeof undo) {
    if (!active) return;
    const result = step(history, { lines: active.lines, song: active.song, channel });
    if (!result) return;
    const [next, snapshot] = result;
    setHistory(next);
    setChannel(snapshot.channel);
    commit({ ...active, lines: snapshot.lines, song: snapshot.song, updated: Date.now() });
  }

  function setPreset(preset: Preset) {
    if (active) commit({ ...active, preset: preset.id, lines: fitLines(active.lines, active.song, preset), updated: Date.now() });
  }

  return {
    projects,
    active,
    channel,
    persistent,
    canUndo: history.past.length > 0,
    canRedo: history.future.length > 0,
    open,
    add,
    addEmpty,
    remove,
    editProject,
    editLines,
    setChannel,
    setPreset,
    undo: () => travel(undo),
    redo: () => travel(redo),
  };
}
