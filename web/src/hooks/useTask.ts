import { useCallback, useState } from 'react';

export interface Status {
  text: string;
  error: boolean;
}

// Ejecuta tareas asíncronas mostrando su progreso y sus errores. `busy`
// permite deshabilitar los botones que lanzarían peticiones duplicadas.
export function useTask() {
  const [status, setStatus] = useState<Status | null>(null);
  const [busy, setBusy] = useState(false);

  const run = useCallback(async (message: string, task: () => Promise<void>) => {
    setBusy(true);
    setStatus({ text: message, error: false });
    try {
      await task();
      // Solo se borra el aviso de progreso, no un mensaje posterior.
      setStatus((current) => (current?.text === message ? null : current));
    } catch (err) {
      setStatus({ text: err instanceof Error ? err.message : String(err), error: true });
    } finally {
      setBusy(false);
    }
  }, []);

  return { status, busy, run };
}
