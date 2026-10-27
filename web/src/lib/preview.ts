// Vista previa de una nota al colocarla en el editor, con el timbre real de
// la voz del chip: el servidor la sintetiza y el navegador conserva en
// memoria cada sonido ya pedido, así que solo la primera vez hay petición.

let context: AudioContext | undefined;
const sounds = new Map<string, Promise<AudioBuffer>>();

export function previewNote(preset: string, voice: number, pitch: number, volume: number): void {
  context ??= new AudioContext();
  const ctx = context;
  const key = `${preset}/${voice}/${pitch}`;
  let sound = sounds.get(key);
  if (!sound) {
    sound = fetch(`/api/preview?preset=${encodeURIComponent(preset)}&voice=${voice}&pitch=${pitch}`)
      .then((res) => {
        if (!res.ok) throw new Error(`Error ${res.status}`);
        return res.arrayBuffer();
      })
      .then((data) => ctx.decodeAudioData(data));
    sounds.set(key, sound);
    sound.catch(() => sounds.delete(key)); // si falla, se reintenta la próxima vez
  }
  sound
    .then((buffer) => {
      const source = new AudioBufferSourceNode(ctx, { buffer });
      source.connect(new GainNode(ctx, { gain: 0.5 * volume })).connect(ctx.destination);
      source.start();
    })
    // La vista previa es accesoria: si falla, la edición sigue sin interrupciones.
    .catch(() => {});
}
