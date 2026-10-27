module github.com/Marcusdeks/FamiGp/appDesktop

go 1.26.0

require github.com/Marcusdeks/FamiGp v0.0.0

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/wailsapp/wails/v3 v3.0.0
	golang.org/x/sys v0.48.0 // indirect
)

// La aplicación de escritorio usa el código del repositorio, no una versión publicada.
replace github.com/Marcusdeks/FamiGp => ../
