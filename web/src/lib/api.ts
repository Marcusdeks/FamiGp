// Cliente de la API de FamiGP.

import type { Preset, ProjectPayload, Song } from '../types';

const OFFLINE = 'El servidor de FamiGP no responde: comprueba que está en marcha (npm run dev o go run ./cmd/famigp).';

// Llama a la API y convierte las respuestas de error en excepciones con el
// mensaje del servidor. Un fallo de red, o un error del proxy de desarrollo
// sin cuerpo JSON, significa que el servidor Go no está arrancado.
async function request(path: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(path, init).catch(() => {
    throw new Error(OFFLINE);
  });
  if (!res.ok) {
    const body: { error?: string } = await res.json().catch(() => ({}));
    throw new Error(body.error ?? (res.status >= 500 ? OFFLINE : `Error ${res.status}`));
  }
  return res;
}

function postJSON(path: string, data: unknown): Promise<Response> {
  return request(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
}

export async function getPresets(): Promise<Preset[]> {
  return (await request('/api/presets')).json();
}

// Analiza un archivo de Guitar Pro. Go serializa las listas vacías como
// null, así que se normalizan para que el resto del cliente no tenga que
// comprobarlo.
//
// El archivo se carga en memoria antes de enviarlo: en la aplicación de
// escritorio, WebView2 no transmite el contenido de los archivos respaldados
// por disco (los del selector o los arrastrados) a su servidor interno.
export async function parseSong(file: File): Promise<Song> {
  const body = new FormData();
  body.append('file', new Blob([await file.arrayBuffer()]), file.name);
  const song: Song = await (await request('/api/song', { method: 'POST', body })).json();
  return {
    ...song,
    bars: song.bars ?? [],
    tracks: song.tracks.map((track) => ({ ...track, notes: track.notes ?? [] })),
  };
}

export async function renderWAV(payload: ProjectPayload): Promise<Blob> {
  return (await postJSON('/api/render', payload)).blob();
}

export async function exportMIDI(payload: ProjectPayload): Promise<Blob> {
  return (await postJSON('/api/midi', payload)).blob();
}
