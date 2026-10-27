// Comando appDesktop empaqueta FamiGP como aplicación de escritorio con
// Wails: la misma interfaz React y la misma API que la versión web, pero en
// una ventana nativa, sin servidor aparte ni navegador.
package main

//go:generate wails3 generate icons -input build/appicon.png -windowsfilename build/windows/icon.ico -macfilename build/darwin/icon.icns
//go:generate wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest build/windows/famigp.exe.manifest -info build/windows/info.json -out rsrc_windows_amd64.syso

import (
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Marcusdeks/FamiGp/internal/server"
	"github.com/Marcusdeks/FamiGp/web"
)

//go:embed build/appicon.png
var icon []byte

func main() {
	static, err := web.Files()
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "FamiGP",
		Description: "Convierte partituras de Guitar Pro en música chiptune de 8 y 16 bits",
		Icon:        icon,
		// La ventana pide la interfaz y la API al mismo manejador HTTP que el
		// servidor web, pero en memoria: no se abre ningún puerto de red.
		Assets: application.AssetOptions{
			Handler:        server.New(static),
			DisableLogging: true,
		},
		Mac: application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "FamiGP",
		Width:            1280,
		Height:           900,
		MinWidth:         900,
		MinHeight:        600,
		BackgroundColour: application.NewRGB(14, 11, 31), // mismo fondo que la interfaz, sin destello blanco al abrir
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
