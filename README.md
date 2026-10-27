# FamiGP

Convierte partituras de **Guitar Pro** en música **chiptune** de 8 y 16 bits y las exporta a **FamiStudio**.

FamiStudio no puede abrir archivos de Guitar Pro. FamiGP lee la partitura, la reproduce con el sonido de chips clásicos (NES, Game Boy, Commodore 64 y recreativas) y genera un MIDI preparado para importarlo en FamiStudio con un canal por voz del chip.

## Funcionalidades

- **Lector de Guitar Pro propio**, sin dependencias externas, para todas las versiones:
  - GP3, GP4 y GP5 (binario)
  - GP6 (`.gpx`: contenedor BCFS con compresión BCFZ)
  - GP7/8 (`.gp`: ZIP con XML)
- **Reproducción fiel a la partitura**: repeticiones, finales alternativos, ligaduras (incluso entre compases), puntillos, tresillos y otros grupos irregulares, cambios de tempo y dinámicas.
- **Cinco chips emulados** con sus particularidades:

  | Chip | Voces | Detalles |
  |---|---|---|
  | NES / Famicom (2A03) | 2 pulsos, triángulo, ruido | Triángulo de 4 bits, volumen de 16 niveles a 60 Hz |
  | Game Boy (DMG) | 2 pulsos, onda, ruido | Tabla de onda de 4 bits, DAC de 8 bits |
  | Commodore 64 (SID) | sierra, pulso, triángulo, ruido | Arpegios PAL a 50 Hz, vibrato |
  | Recreativa FM (YM2151) | 3 voces FM, ruido | Síntesis FM de dos operadores |
  | Recreativa Namco (WSG) | 3 tablas de onda, ruido | El sonido de Pac-Man y Galaga |

- **Editor piano roll** por canal: añade, mueve y borra notas, y acorta o alarga cualquiera arrastrando sus bordes, con la duración en segundos a la vista. Rejilla ajustable o libre (también con Alt), deshacer y rehacer (Ctrl+Z / Ctrl+Y). Las notas de los demás canales se muestran atenuadas como guía, y el canal de ruido usa filas con nombre (bombo, caja, charles…).
- **Vista previa con la voz real del chip**: al colocar o tocar una nota suena exactamente como en el resultado, sintetizada por el servidor.
- **Transposición y silencio por canal**: cada línea sube o baja por semitonos u octavas, o se silencia con un clic, sin modificar las notas guardadas.
- **Hasta 5 proyectos** guardados en el navegador (IndexedDB), con cambio rápido entre ellos. Se pueden añadir varios archivos a la vez o crear un proyecto vacío para componer desde cero.
- **Línea de tiempo de la canción**: una franja por canal con sus notas en miniatura y una regla en segundos. Clic para saltar a un punto; arrastrar para seleccionar un tramo y cortarlo (principio, final o un trozo intermedio), con deshacer.
- **Arpegios automáticos**: como en el hardware original, cada canal es monofónico; los acordes se alternan a la frecuencia de refresco del chip.
- **Percusión con ruido LFSR**: bombo, caja, charles y platos se imitan con el generador de ruido de la NES, con cuerpo tonal en bombo y toms.
- **Reproductor con barra de vida**: escucha la canción completa o solo el canal que editas. El progreso y el volumen se muestran como barras de vida; al pasar el ratón se ve el segundo exacto al que se saltará. Cualquier cambio detiene el audio, que ya no corresponde a lo editado.
- **Exportación** a WAV y a MIDI para FamiStudio.

## Tecnologías

| Parte | Tecnologías |
|---|---|
| Backend | Go (solo biblioteca estándar) |
| Frontend | React 19, TypeScript, Tailwind CSS 4, Vite |
| APIs del navegador | Canvas, Web Audio, IndexedDB |
| Calidad | Tests y *fuzzing* de Go, Vitest, ESLint |
| Escritorio | Wails 3 (WebView2) |
| Despliegue | Docker, GitHub Actions |

## Uso

Requiere [Go 1.24](https://go.dev/dl/) y [Node.js 22](https://nodejs.org/) o superiores.

```sh
git clone https://github.com/Marcusdeks/FamiGp.git
cd FamiGp
cd web && npm install && npm run build && cd ..
go run ./cmd/famigp
```

Abre <http://localhost:8080> y sigue estos pasos:

1. Sube tu partitura: se crea un proyecto.
2. Elige el chip.
3. Decide qué pista toca cada canal y transpórtala si hace falta.
4. Retoca las notas en el editor o escribe líneas nuevas en los canales vacíos.
5. Escucha el resultado o descárgalo.

Para usar otro puerto: `go run ./cmd/famigp -addr localhost:3000`.

### Desarrollo con recarga automática

Un solo comando arranca la interfaz y el servidor, y los cambios se ven al guardar sin reiniciar nada:

```sh
cd web
npm run dev
```

Abre <http://localhost:8080>. Vite actualiza la interfaz al instante y reenvía las llamadas a `/api` al servidor Go, que en desarrollo escucha en el puerto 8081 y que [Air](https://github.com/air-verse/air) recompila y reinicia en un par de segundos al guardar un `.go`. Air se descarga y ejecuta con `go run`, sin instalar nada. Con Ctrl+C se paran los dos.

### Aplicación de escritorio (Windows)

La carpeta `appDesktop/` empaqueta la misma interfaz y la misma API en un ejecutable nativo con [Wails 3](https://v3.wails.io/), sin servidor ni navegador aparte:

```powershell
cd appDesktop
.\build.ps1        # genera appDesktop\bin\FamiGP.exe
```

La ventana usa WebView2, que viene de serie en Windows 10 y 11. Los proyectos se guardan en `%APPDATA%\FamiGP.exe`. Es un módulo de Go independiente, de modo que Wails solo es dependencia de la versión de escritorio. Tras cambiar `build/appicon.png`, `go generate` (con la CLI `wails3` instalada) regenera los iconos y los recursos de Windows.

### Con Docker

```sh
docker build -t famigp .
docker run --rm -p 8080:8080 famigp
```

### Importar en FamiStudio

1. Pulsa **MIDI para FamiStudio** con el chip **NES / Famicom**.
2. En FamiStudio: *File → Import → MIDI*.
3. Asigna cada pista del MIDI (`Pulso 1`, `Pulso 2`, `Triángulo`, `Ruido`) a su canal.

Los canales de la NES son monofónicos, así que conviene que cada pista lleve una sola línea melódica. Los acordes pueden recrearse con los arpegios de FamiStudio.

## Arquitectura

```
cmd/famigp/          Punto de entrada: servidor HTTP con cierre ordenado
internal/song/       Modelo musical neutro (notas en ticks y mapa de tempo)
internal/guitarpro/  Lectores de GP3-5, GP6 y GP7/8 → modelo común
internal/chip/       Presets de chips, síntesis y codificación WAV
internal/midi/       Escritor de MIDI estándar (SMF formato 1)
internal/server/     API REST sin estado y validación de proyectos
web/                 Interfaz React + TypeScript; su compilación se incrusta en el binario
  src/components/    Componentes de la interfaz (proyectos, chips, canales, editor, reproductor)
  src/hooks/         Estado: proyectos con deshacer, audio y tareas asíncronas
  src/lib/           Lógica sin React: piano roll sobre canvas, corte de tramos, API, IndexedDB, tiempos
```

El piano roll es una clase imperativa sobre Canvas que React controla mediante un componente envoltorio: dibuja solo la zona visible y entrega un array nuevo de notas al terminar cada gesto, sin mutar el estado de React. El historial de deshacer guarda instantáneas inmutables de las líneas, que comparten por referencia las partes que no cambian.

El flujo es: **archivo → `guitarpro.Parse` → `song.Song` → editor en el navegador → `chip.Render` / `midi.Write`**. Cada paquete depende solo del modelo `song`, de modo que añadir un formato de entrada o un chip nuevo no afecta al resto. El servidor no guarda nada: el navegador conserva los proyectos y envía el estado editado en cada síntesis.

### API

| Método | Ruta | Entrada | Salida |
|---|---|---|---|
| `GET` | `/api/presets` | — | Chips disponibles y sus voces |
| `POST` | `/api/song` | multipart `file` | Canción completa (notas, tempo, compases) y asignación sugerida por chip |
| `POST` | `/api/render` | proyecto JSON | `audio/wav` |
| `POST` | `/api/midi` | proyecto JSON | `audio/midi` |
| `GET` | `/api/preview?preset=nes&voice=3&pitch=38` | chip, voz y altura | `audio/wav` de una nota suelta |

Proyecto JSON: una línea de notas por voz del chip, con su transposición en semitonos.

```json
{
  "title": "Mi canción",
  "preset": "nes",
  "tempos": [{ "tick": 0, "bpm": 120 }],
  "length": 15360,
  "lines": [
    { "transpose": 12, "notes": [{ "start": 0, "duration": 480, "pitch": 64, "velocity": 95 }] },
    { "transpose": 0, "notes": [] },
    { "transpose": 0, "notes": [] },
    { "transpose": 0, "notes": [{ "start": 0, "duration": 240, "pitch": 36, "velocity": 95 }] }
  ]
}
```

Los tiempos van en ticks (960 por negra). En el canal de ruido, `pitch` es el instrumento de percusión General MIDI (36 = bombo, 38 = caja, 42 = charles…).

## Calidad

```sh
go vet ./...
go test ./...
go test ./internal/guitarpro -run '^$' -fuzz FuzzParse -fuzztime 60s

cd web
npm run lint   # ESLint, incluidas las reglas de hooks de React
npm test       # Vitest: lógica de proyectos, historial, cortes y tiempos
npm run build  # comprobación de tipos de TypeScript y compilación
```

- El lector se validó contra los 126 archivos de prueba de [PyGuitarPro](https://github.com/Perlence/PyGuitarPro) y [alphaTab](https://github.com/CoderLine/alphaTab), comparando nota a nota con alphaTab como referencia.
- El *fuzzing* garantiza que ningún archivo dañado o malicioso provoque un fallo del servidor. Hay límites frente a bombas ZIP y a la amplificación por repeticiones.
- La interfaz se probó de extremo a extremo en Chrome: creación y cambio de proyectos, persistencia tras recargar, límite de 5, edición en el piano roll con deshacer y rehacer, transposición y parada del audio al editar.
- La aplicación de escritorio se probó con la misma batería de pruebas, ejecutada dentro de su ventana WebView2, incluidas las descargas y el arrastre de archivos desde el disco.
- La integración continua (GitHub Actions) comprueba el backend (formato, análisis estático, tests con detector de condiciones de carrera y *fuzzing*), el frontend (lint, tests, tipos y compilación), compila el ejecutable de Windows y construye la imagen Docker.

## Limitaciones conocidas

- Los armónicos suenan a la altura del traste pisado, no a la del armónico.
- No se interpretan bends, slides ni vibratos escritos en la partitura; el vibrato de algunos chips forma parte del timbre del preset.
- Los proyectos se guardan en el navegador: no se comparten entre equipos ni navegadores distintos.
- El editor está pensado para ratón; en pantallas táctiles, desplazar la vista puede crear notas.

## Aviso legal

FamiGP es un proyecto independiente sin relación con Arobas Music (Guitar Pro), FamiStudio, Nintendo, Commodore, Yamaha, Namco ni ninguna otra empresa mencionada. Sus nombres y marcas se citan únicamente para indicar compatibilidad. El lector de Guitar Pro es una implementación propia para la interoperabilidad de formatos y el sonido de los chips se sintetiza desde cero: el proyecto no incluye código, muestras de audio ni archivos de terceros. El uso de partituras protegidas por derechos de autor es responsabilidad de quien las convierte.
