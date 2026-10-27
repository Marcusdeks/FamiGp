import { useState } from 'react';
import { MAX_PROJECTS } from '../lib/project';
import { durationSeconds, formatTime } from '../lib/timing';
import type { Project } from '../types';
import { Panel } from './Panel';

interface Props {
  projects: Project[];
  active: Project | null;
  onOpen(project: Project): void;
  onRemove(project: Project): void;
  onFiles(files: File[]): void;
  onEmpty(): void;
}

// Lista de proyectos guardados y zona para añadir nuevos desde archivos.
export function ProjectList({ projects, active, onOpen, onRemove, onFiles, onEmpty }: Props) {
  const [dragging, setDragging] = useState(false);

  return (
    <Panel step={1} title="Proyectos" badge={`${projects.length}/${MAX_PROJECTS}`}>
      {projects.length > 0 && (
        <ul className="mb-3 flex flex-wrap gap-2.5">
          {projects.map((project) => {
            const selected = project === active ? 'border-gold bg-gold/12' : '';
            return (
              <li key={project.id} className="flex max-w-full">
                <button type="button" className={`btn max-w-64 truncate shadow-none ${selected}`} onClick={() => onOpen(project)}>
                  {project.name}
                </button>
                <button
                  type="button"
                  title="Eliminar proyecto"
                  aria-label={`Eliminar ${project.name}`}
                  className={`btn border-l-0 text-muted shadow-none ${selected}`}
                  onClick={() => onRemove(project)}
                >
                  ×
                </button>
              </li>
            );
          })}
        </ul>
      )}

      {active && (
        <p className="mb-4 text-muted">
          {[
            active.song.artist,
            formatTime(durationSeconds(active.song.tempos, active.song.length)),
            `${active.song.tracks.length} pistas`,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      )}

      <div className="flex flex-wrap items-stretch gap-3">
        {/* El input ocupa toda la zona para que el clic y el teclado funcionen. */}
        <label
          className={`relative grid flex-1 basis-72 cursor-pointer gap-1.5 border-3 border-dashed px-4 py-5 text-center transition-colors focus-within:border-cyan hover:border-cyan ${
            dragging ? 'border-cyan bg-cyan/8' : 'border-line'
          }`}
          onDragOver={(e) => {
            e.preventDefault();
            setDragging(true);
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault();
            setDragging(false);
            if (e.dataTransfer.files.length) onFiles([...e.dataTransfer.files]);
          }}
        >
          <input
            type="file"
            multiple
            accept=".gp,.gpx,.gp3,.gp4,.gp5"
            className="absolute inset-0 cursor-pointer opacity-0"
            onChange={(e) => {
              if (e.target.files?.length) onFiles([...e.target.files]);
              e.target.value = ''; // permite volver a elegir los mismos archivos
            }}
          />
          <strong>Añadir proyectos: arrastra archivos aquí o haz clic</strong>
          <small className="text-muted">
            Guitar Pro 3, 4, 5, 6 y 7/8 (.gp3 .gp4 .gp5 .gpx .gp) · Hasta {MAX_PROJECTS} proyectos, guardados en este navegador
          </small>
        </label>
        <button type="button" className="btn" title="Crea un proyecto sin archivo para componer desde cero" onClick={onEmpty}>
          + Proyecto vacío
        </button>
      </div>
    </Panel>
  );
}
