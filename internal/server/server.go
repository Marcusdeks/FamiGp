// Package server expone la conversión mediante una API HTTP y sirve la
// interfaz web. Es sin estado: los proyectos viven en el navegador y cada
// petición incluye todo lo necesario.
package server

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Marcusdeks/FamiGp/internal/chip"
	"github.com/Marcusdeks/FamiGp/internal/guitarpro"
	"github.com/Marcusdeks/FamiGp/internal/midi"
	"github.com/Marcusdeks/FamiGp/internal/song"
)

const maxUpload = 20 << 20 // 20 MB, muy por encima de cualquier partitura o proyecto real

// New crea el manejador HTTP con la API y los archivos estáticos de static.
// Con static nil solo se sirve la API, que es lo que ocurre en desarrollo,
// cuando la interfaz la sirve Vite.
func New(static fs.FS) http.Handler {
	mux := http.NewServeMux()
	if static != nil {
		mux.Handle("GET /", http.FileServerFS(static))
	}
	mux.HandleFunc("GET /api/presets", handlePresets)
	mux.HandleFunc("POST /api/song", handleSong)
	mux.HandleFunc("POST /api/render", handleRender)
	mux.HandleFunc("POST /api/midi", handleMIDI)
	mux.HandleFunc("GET /api/preview", handlePreview)
	return mux
}

// presetView es la vista pública de un preset para construir el menú.
type presetView struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Voices      []voiceView `json:"voices"`
}

type voiceView struct {
	Name  string `json:"name"`
	Drums bool   `json:"drums"`
}

func handlePresets(w http.ResponseWriter, _ *http.Request) {
	views := make([]presetView, len(chip.Presets))
	for i, p := range chip.Presets {
		views[i] = presetView{ID: p.ID, Name: p.Name, Description: p.Description}
		for _, v := range p.Voices {
			views[i].Voices = append(views[i].Voices, voiceView{v.Name, v.Drums()})
		}
	}
	writeJSON(w, http.StatusOK, views)
}

// songView es la canción completa, que el cliente guarda como base del
// proyecto, más una asignación de pistas a canales propuesta para cada preset.
type songView struct {
	*song.Song
	Defaults map[string][]int `json:"defaults"`
}

// handleSong analiza el archivo de Guitar Pro del campo "file".
func handleSong(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, badRequest(errors.New("falta el archivo de Guitar Pro o supera los 20 MB")))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, badRequest(err))
		return
	}
	s, err := guitarpro.Parse(data)
	if err != nil {
		writeError(w, badRequest(err))
		return
	}
	s.Title = cmp.Or(s.Title, strings.TrimSuffix(header.Filename, filepath.Ext(header.Filename)))
	view := songView{Song: s, Defaults: make(map[string][]int, len(chip.Presets))}
	for _, p := range chip.Presets {
		view.Defaults[p.ID] = chip.DefaultMapping(s, p)
	}
	writeJSON(w, http.StatusOK, view)
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	s, p, parts, err := readProject(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	samples, err := chip.Render(s, p, parts)
	if err != nil {
		writeError(w, badRequest(err))
		return
	}
	var buf bytes.Buffer
	if err := chip.WriteWAV(&buf, samples); err != nil {
		writeError(w, err)
		return
	}
	send(w, "audio/wav", buf.Bytes())
}

// handlePreview devuelve una nota suelta con el timbre de una voz del chip
// (?preset=nes&voice=3&pitch=38) para oírla al colocarla en el editor.
func handlePreview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, ok := chip.Find(q.Get("preset"))
	voice, errVoice := strconv.Atoi(q.Get("voice"))
	pitch, errPitch := strconv.Atoi(q.Get("pitch"))
	if !ok || errVoice != nil || errPitch != nil {
		writeError(w, badRequest(errors.New("parámetros de vista previa inválidos")))
		return
	}
	samples, err := chip.Preview(p, voice, pitch)
	if err != nil {
		writeError(w, badRequest(err))
		return
	}
	var buf bytes.Buffer
	if err := chip.WriteWAV(&buf, samples); err != nil {
		writeError(w, err)
		return
	}
	// La misma nota del mismo chip suena siempre igual: el navegador puede reutilizarla.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	send(w, "audio/wav", buf.Bytes())
}

func handleMIDI(w http.ResponseWriter, r *http.Request) {
	s, _, parts, err := readProject(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	var buf bytes.Buffer
	if err := midi.Write(&buf, s, parts); err != nil {
		writeError(w, err)
		return
	}
	send(w, "audio/midi", buf.Bytes())
}

// httpError asocia un error a un código de estado HTTP.
type httpError struct {
	status int
	err    error
}

func (e httpError) Error() string { return e.err.Error() }
func (e httpError) Unwrap() error { return e.err }

func badRequest(err error) error { return httpError{http.StatusBadRequest, err} }

// writeError responde con el mensaje del error si es del cliente; los
// errores internos se registran y se ocultan tras un mensaje genérico.
func writeError(w http.ResponseWriter, err error) {
	var he httpError
	if errors.As(err, &he) {
		writeJSON(w, he.status, map[string]string{"error": he.Error()})
		return
	}
	log.Printf("error interno: %v", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "error interno del servidor"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("no se pudo escribir la respuesta: %v", err)
	}
}

func send(w http.ResponseWriter, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprint(len(data)))
	if _, err := w.Write(data); err != nil {
		log.Printf("no se pudo enviar la respuesta: %v", err)
	}
}
