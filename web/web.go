// Package web incrusta la interfaz compilada (web/dist/app, generada con
// `npm run build`) para distribuir la aplicación como un único ejecutable.
package web

import (
	"embed"
	"errors"
	"io/fs"
)

// dist incluye dist/.gitkeep para que compile aunque la interfaz no se
// haya generado todavía.
//
//go:embed all:dist
var dist embed.FS

// ErrNotBuilt indica que falta la interfaz compilada.
var ErrNotBuilt = errors.New("la interfaz no está compilada: ejecuta `npm run build` en web/ o usa `npm run dev`")

// Files devuelve los archivos de la interfaz lista para servir.
func Files() (fs.FS, error) {
	files, err := fs.Sub(dist, "dist/app")
	if err != nil {
		return nil, err
	}
	if _, err := fs.Stat(files, "index.html"); err != nil {
		return nil, ErrNotBuilt
	}
	return files, nil
}
