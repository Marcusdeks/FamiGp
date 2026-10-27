import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

// En desarrollo, Vite sirve la interfaz en el puerto 8080 con recarga en
// caliente y reenvía las llamadas a la API al servidor Go, que en ese modo
// escucha en el 8081 (ver .air.toml). En producción, `vite build` genera
// dist/app, que se incrusta en el binario de Go (dist/.gitkeep se conserva
// para que Go compile aunque la interfaz no se haya generado).
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: 'dist/app', emptyOutDir: true },
  server: {
    port: 8080,
    strictPort: true,
    proxy: { '/api': 'http://localhost:8081' },
  },
});
