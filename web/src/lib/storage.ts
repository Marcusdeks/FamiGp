// Persistencia de proyectos en IndexedDB: se conservan en este navegador
// entre sesiones. Todas las funciones devuelven promesas que fallan si el
// almacenamiento no está disponible (por ejemplo, en modo privado).

import type { Project } from '../types';

const DB_NAME = 'famigp';
const STORE = 'projects';

let database: Promise<IDBDatabase> | undefined;

function open(): Promise<IDBDatabase> {
  database ??= new Promise((resolve, reject) => {
    const req = indexedDB.open(DB_NAME, 1);
    req.onupgradeneeded = () => req.result.createObjectStore(STORE, { keyPath: 'id' });
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
  return database;
}

// Ejecuta una operación sobre el almacén y resuelve con su resultado.
async function request<T>(mode: IDBTransactionMode, operation: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await open();
  return new Promise((resolve, reject) => {
    const req = operation(db.transaction(STORE, mode).objectStore(STORE));
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error);
  });
}

export const loadProjects = () => request<Project[]>('readonly', (store) => store.getAll());
export const saveProject = (project: Project) => request('readwrite', (store) => store.put(project));
export const deleteProject = (id: string) => request('readwrite', (store) => store.delete(id));
