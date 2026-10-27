// Package chip emula el sonido de los chips de audio de consolas y
// recreativas de 8/16 bits y convierte una canción en audio.
package chip

import "math"

// Wave es la forma de onda que genera un oscilador.
type Wave int

const (
	Pulse    Wave = iota // onda cuadrada con ciclo de trabajo variable (NES, Game Boy)
	Triangle             // triángulo escalonado de 4 bits (NES)
	Saw                  // diente de sierra (SID del Commodore 64)
	Table                // tabla de onda de 4 bits definida por el usuario (Game Boy, Namco)
	FM                   // síntesis FM de dos operadores (Yamaha YM2151)
	Noise                // ruido de registro de desplazamiento (LFSR), para percusión
)

// Voice describe un canal del chip: forma de onda, timbre y envolvente ADSR.
type Voice struct {
	Name    string
	Wave    Wave
	Duty    float64   // fracción del ciclo en alto (Pulse)
	Table   []float64 // muestras en [-1, 1] (Table)
	Ratio   float64   // frecuencia del modulador respecto a la portadora (FM)
	Index   float64   // profundidad de modulación (FM)
	Vibrato float64   // profundidad del vibrato en semitonos (0 = sin vibrato)
	Attack  float64   // segundos hasta el volumen máximo
	Decay   float64   // segundos hasta el nivel de sostenimiento
	Sustain float64   // nivel mientras la nota se mantiene (0-1)
	Release float64   // segundos de caída tras soltar la nota
}

// Drums indica si la voz es el canal de percusión.
func (v Voice) Drums() bool { return v.Wave == Noise }

// Preset agrupa las voces de un chip concreto y sus particularidades.
type Preset struct {
	ID          string
	Name        string
	Description string
	Voices      []Voice
	FrameRate   float64 // frecuencia de refresco del motor de sonido (arpegios y envolventes)
	VolumeSteps int     // niveles de volumen del hardware (0 = continuo)
	Bits        int     // resolución del DAC de salida (0 = sin reducir)
}

// Presets es el catálogo de chips disponibles, en el orden en que se muestran.
// Todos comparten la disposición de tres voces tonales y una de ruido, de
// modo que las líneas de un proyecto se conservan al cambiar de chip.
var Presets = []Preset{
	{
		ID:          "nes",
		Name:        "NES / Famicom",
		Description: "Ricoh 2A03: dos pulsos, triángulo y ruido. El sonido de FamiStudio.",
		FrameRate:   60, VolumeSteps: 15,
		Voices: []Voice{
			{Name: "Pulso 1", Wave: Pulse, Duty: 0.25, Decay: 0.3, Sustain: 0.6, Release: 0.05},
			{Name: "Pulso 2", Wave: Pulse, Duty: 0.5, Decay: 0.3, Sustain: 0.5, Release: 0.05},
			{Name: "Triángulo", Wave: Triangle, Sustain: 1, Release: 0.02},
			{Name: "Ruido", Wave: Noise},
		},
	},
	{
		ID:          "gameboy",
		Name:        "Game Boy",
		Description: "DMG-01: pulsos, canal de onda de 4 bits y ruido. Chiptune portátil.",
		FrameRate:   64, VolumeSteps: 15, Bits: 8,
		Voices: []Voice{
			{Name: "Pulso 1", Wave: Pulse, Duty: 0.5, Decay: 0.4, Sustain: 0.5, Release: 0.08},
			{Name: "Pulso 2", Wave: Pulse, Duty: 0.125, Decay: 0.4, Sustain: 0.4, Release: 0.08},
			{Name: "Onda", Wave: Table, Table: table4(func(x float64) float64 { return math.Sin(2*math.Pi*x) + 0.5*math.Sin(4*math.Pi*x) }), Sustain: 1, Release: 0.04},
			{Name: "Ruido", Wave: Noise},
		},
	},
	{
		ID:          "c64",
		Name:        "Commodore 64",
		Description: "MOS SID: sierra, pulso y triángulo con arpegios PAL a 50 Hz.",
		FrameRate:   50, Bits: 12,
		Voices: []Voice{
			{Name: "Sierra", Wave: Saw, Vibrato: 0.15, Attack: 0.01, Decay: 0.2, Sustain: 0.7, Release: 0.15},
			{Name: "Pulso", Wave: Pulse, Duty: 0.4, Attack: 0.005, Decay: 0.25, Sustain: 0.5, Release: 0.12},
			{Name: "Triángulo", Wave: Triangle, Attack: 0.005, Decay: 0.1, Sustain: 0.9, Release: 0.1},
			{Name: "Ruido", Wave: Noise},
		},
	},
	{
		ID:          "arcade-fm",
		Name:        "Recreativa FM",
		Description: "Yamaha YM2151 (CPS1, System 16): metales y bajos FM de 16 bits.",
		FrameRate:   60, Bits: 10,
		Voices: []Voice{
			{Name: "FM Lead", Wave: FM, Ratio: 1, Index: 1.8, Vibrato: 0.2, Attack: 0.01, Decay: 0.4, Sustain: 0.6, Release: 0.15},
			{Name: "FM Brass", Wave: FM, Ratio: 2, Index: 1.2, Attack: 0.02, Decay: 0.3, Sustain: 0.5, Release: 0.12},
			{Name: "FM Bass", Wave: FM, Ratio: 0.5, Index: 2.5, Decay: 0.25, Sustain: 0.4, Release: 0.08},
			{Name: "Ruido", Wave: Noise},
		},
	},
	{
		ID:          "arcade-wsg",
		Name:        "Recreativa Namco",
		Description: "Namco WSG (Pac-Man, Galaga): tres voces de tabla de onda de 4 bits y ruido para la percusión.",
		FrameRate:   60, VolumeSteps: 15, Bits: 8,
		Voices: []Voice{
			{Name: "Onda 1", Wave: Table, Table: table4(func(x float64) float64 { return math.Sin(2 * math.Pi * x) }), Decay: 0.3, Sustain: 0.7, Release: 0.06},
			{Name: "Onda 2", Wave: Table, Table: table4(func(x float64) float64 { return 2*x - 1 }), Decay: 0.3, Sustain: 0.5, Release: 0.06},
			{Name: "Onda 3", Wave: Table, Table: table4(func(x float64) float64 {
				return math.Copysign(math.Pow(math.Abs(math.Sin(2*math.Pi*x)), 0.3), math.Sin(2*math.Pi*x))
			}), Sustain: 0.9, Release: 0.04},
			{Name: "Ruido", Wave: Noise},
		},
	},
}

// Find devuelve el preset con el identificador dado.
func Find(id string) (Preset, bool) {
	for _, p := range Presets {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}

// table4 muestrea una función periódica en 32 pasos cuantizados a 4 bits,
// igual que la memoria de onda de los chips de la época.
func table4(f func(x float64) float64) []float64 {
	raw := make([]float64, 32)
	peak := 0.0
	for i := range raw {
		raw[i] = f(float64(i) / 32)
		peak = max(peak, math.Abs(raw[i]))
	}
	for i, v := range raw {
		raw[i] = math.Round((v/peak+1)*7.5)/7.5 - 1
	}
	return raw
}
