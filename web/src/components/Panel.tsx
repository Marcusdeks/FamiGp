import type { ReactNode } from 'react';

interface Props {
  step: number;
  title: string;
  badge?: string;
  children: ReactNode;
}

// Sección numerada de la interfaz con el marco pixelado común.
export function Panel({ step, title, badge, children }: Props) {
  return (
    <section className="min-w-0 border-3 border-line bg-panel p-5 shadow-pixel">
      <h2 className="mb-4 flex flex-wrap items-center gap-3 font-pixel text-sm text-gold">
        <span className="bg-famired px-2 py-1.5 text-ink">{step}</span>
        {title}
        {badge && <span className="border-2 border-line px-2 py-1 text-[10px] text-cyan">{badge}</span>}
      </h2>
      {children}
    </section>
  );
}
