// Convierte un WAV a MP3 en un Web Worker, sin bloquear la interfaz.

interface WorkerReply {
  blob?: Blob;
  error?: string;
}

export async function encodeMp3(wav: Blob): Promise<Blob> {
  const buffer = await wav.arrayBuffer();
  const worker = new Worker(new URL('./mp3.worker.ts', import.meta.url), { type: 'module' });
  try {
    return await new Promise<Blob>((resolve, reject) => {
      worker.onmessage = (e: MessageEvent<WorkerReply>) => (e.data.blob ? resolve(e.data.blob) : reject(new Error(e.data.error)));
      worker.onerror = () => reject(new Error('No se pudo codificar el MP3'));
      // El búfer se transfiere en lugar de copiarse: no se duplica en memoria.
      worker.postMessage(buffer, [buffer]);
    });
  } finally {
    worker.terminate();
  }
}
