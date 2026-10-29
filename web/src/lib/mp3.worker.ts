// Web Worker que codifica a MP3 en un hilo aparte. Recibe el WAV como
// ArrayBuffer y responde con el MP3 o con el mensaje de error.

import { wavToMp3 } from './mp3';

self.onmessage = (e: MessageEvent<ArrayBuffer>) => {
  try {
    self.postMessage({ blob: wavToMp3(e.data) });
  } catch (err) {
    self.postMessage({ error: err instanceof Error ? err.message : String(err) });
  }
};
