// Iconos en pixel art dibujados con rectángulos SVG (crispEdges mantiene los
// bordes nítidos a cualquier escala). Toman el color del texto (currentColor).

type Pixel = [x: number, y: number, width: number, height: number];

function PixelIcon({ pixels, width, height, scale = 2 }: { pixels: Pixel[]; width: number; height: number; scale?: number }) {
  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      width={width * scale}
      height={height * scale}
      fill="currentColor"
      shapeRendering="crispEdges"
      aria-hidden="true"
    >
      {pixels.map(([x, y, w, h]) => (
        <rect key={`${x},${y}`} x={x} y={y} width={w} height={h} />
      ))}
    </svg>
  );
}

// Altavoz de 11×9 píxeles: con ondas si suena, con una X si está silenciado.
const SPEAKER: Pixel[] = [[0, 3, 2, 3], [2, 2, 1, 5], [3, 1, 1, 7], [4, 0, 1, 9]];
const WAVES: Pixel[] = [[6, 3, 1, 3], [8, 1, 1, 7]];
const CROSS: Pixel[] = [[6, 2, 1, 1], [7, 3, 1, 1], [8, 4, 1, 1], [9, 5, 1, 1], [10, 6, 1, 1], [10, 2, 1, 1], [9, 3, 1, 1], [7, 5, 1, 1], [6, 6, 1, 1]];

export function SpeakerIcon({ muted }: { muted: boolean }) {
  return <PixelIcon pixels={[...SPEAKER, ...(muted ? CROSS : WAVES)]} width={11} height={9} />;
}

// Corazón de 7×6 píxeles, como el de las barras de vida de los juegos de la época.
export function HeartIcon() {
  return (
    <PixelIcon
      pixels={[[1, 0, 2, 1], [4, 0, 2, 1], [0, 1, 7, 2], [1, 3, 5, 1], [2, 4, 3, 1], [3, 5, 1, 1]]}
      width={7}
      height={6}
      scale={3}
    />
  );
}
