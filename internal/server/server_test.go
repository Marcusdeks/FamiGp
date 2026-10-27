package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Marcusdeks/FamiGp/internal/song"
)

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(New(fstest.MapFS{"index.html": {Data: []byte("<h1>FamiGP</h1>")}}))
	t.Cleanup(srv.Close)
	return srv
}

// gp7File empaqueta la partitura de prueba del lector como archivo .gp.
func gp7File(t *testing.T) []byte {
	t.Helper()
	xmlData, err := os.ReadFile("../guitarpro/testdata/score.gpif")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("Content/score.gpif")
	w.Write(xmlData)
	zw.Close()
	return buf.Bytes()
}

// upload envía un archivo en un formulario multipart a /api/song.
func upload(t *testing.T, url string, file []byte) *http.Response {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "cancion.gp")
	fw.Write(file)
	mw.Close()
	res, err := http.Post(url+"/api/song", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func post(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(body)
	res, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

// validProject es un proyecto mínimo para el chip NES: un Do en el primer
// pulso, transpuesto una octava, y un bombo.
func validProject() projectRequest {
	return projectRequest{
		Title:  "Prueba",
		Preset: "nes",
		Tempos: []song.Tempo{{Tick: 0, BPM: 120}},
		Length: 4 * song.PPQ,
		Lines: []lineRequest{
			{Transpose: 12, Notes: []song.Note{{Start: song.PPQ, Duration: song.PPQ, Pitch: 60, Velocity: 100}, {Start: 0, Duration: song.PPQ, Pitch: 62, Velocity: 100}}},
			{},
			{},
			{Notes: []song.Note{{Start: 0, Duration: 240, Pitch: 36, Velocity: 100}}},
		},
	}
}

func TestStaticAndPresets(t *testing.T) {
	srv := newServer(t)
	res, err := http.Get(srv.URL + "/")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %v, %v", res.StatusCode, err)
	}
	res.Body.Close()

	res, err = http.Get(srv.URL + "/api/presets")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var presets []presetView
	if err := json.NewDecoder(res.Body).Decode(&presets); err != nil || len(presets) == 0 {
		t.Fatalf("presets = %v, %v", presets, err)
	}
	if presets[0].ID != "nes" || !presets[0].Voices[3].Drums {
		t.Errorf("primer preset inesperado: %+v", presets[0])
	}
}

func TestSong(t *testing.T) {
	srv := newServer(t)
	res := upload(t, srv.URL, gp7File(t))
	var view struct {
		song.Song
		Defaults map[string][]int `json:"defaults"`
	}
	if err := json.NewDecoder(res.Body).Decode(&view); err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("estado %d, %v", res.StatusCode, err)
	}
	if view.Title != "Prueba" || len(view.Tracks) != 2 || !view.Tracks[1].Percussion || len(view.Tracks[1].Notes) != 4 {
		t.Errorf("canción inesperada: %+v", view.Song)
	}
	if len(view.Bars) != 2 || len(view.Tempos) == 0 {
		t.Errorf("faltan compases o tempo: %+v", view.Song)
	}
	if got := view.Defaults["nes"]; len(got) != 4 || got[0] != 0 || got[3] != 1 {
		t.Errorf("asignación por defecto NES = %v", got)
	}
}

func TestRenderAndMIDI(t *testing.T) {
	srv := newServer(t)
	tests := []struct{ path, contentType, magic string }{
		{"/api/render", "audio/wav", "RIFF"},
		{"/api/midi", "audio/midi", "MThd"},
	}
	for _, tt := range tests {
		res := post(t, srv.URL+tt.path, validProject())
		var head [4]byte
		res.Body.Read(head[:])
		if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != tt.contentType || string(head[:]) != tt.magic {
			t.Errorf("%s: estado %d, tipo %q, cabecera %q", tt.path, res.StatusCode, res.Header.Get("Content-Type"), head)
		}
	}
}

// Las notas llegan en el orden en que se editaron: el servidor debe
// transponerlas y ordenarlas por inicio antes de sintetizar.
func TestReadProjectTransposesAndSorts(t *testing.T) {
	body, _ := json.Marshal(validProject())
	req := httptest.NewRequest(http.MethodPost, "/api/render", bytes.NewReader(body))
	s, p, parts, err := readProject(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatal(err)
	}
	notes := parts[0].Notes
	if p.ID != "nes" || notes[0].Start != 0 || notes[0].Pitch != 74 || notes[1].Pitch != 72 {
		t.Errorf("notas = %+v", notes)
	}
	if !parts[3].Percussion || s.Length != 4*song.PPQ {
		t.Errorf("partes = %+v, duración = %d", parts, s.Length)
	}
}

func TestClientErrors(t *testing.T) {
	srv := newServer(t)
	modify := func(change func(*projectRequest)) projectRequest {
		p := validProject()
		change(&p)
		return p
	}
	tests := []struct {
		name    string
		body    any
		message string
	}{
		{"JSON ilegible", "{", "ilegible"},
		{"chip desconocido", modify(func(p *projectRequest) { p.Preset = "zx" }), "chip desconocido"},
		{"canales de menos", modify(func(p *projectRequest) { p.Lines = p.Lines[:2] }), "canales"},
		{"sin tempo", modify(func(p *projectRequest) { p.Tempos = nil }), "tempo"},
		{"tempo desordenado", modify(func(p *projectRequest) { p.Tempos = append(p.Tempos, song.Tempo{Tick: 0, BPM: 0}) }), "tempo"},
		{"transposición fuera de rango", modify(func(p *projectRequest) { p.Lines[0].Transpose = 100 }), "rango MIDI"},
		{"duración nula", modify(func(p *projectRequest) { p.Lines[3].Notes[0].Duration = 0 }), "duración"},
		{"intensidad inválida", modify(func(p *projectRequest) { p.Lines[3].Notes[0].Velocity = 0 }), "intensidad"},
		{"demasiado larga", modify(func(p *projectRequest) { p.Length = 1 << 29 }), "máximo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var res *http.Response
			if raw, ok := tt.body.(string); ok {
				r, err := http.Post(srv.URL+"/api/render", "application/json", strings.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { r.Body.Close() })
				res = r
			} else {
				res = post(t, srv.URL+"/api/render", tt.body)
			}
			var body map[string]string
			json.NewDecoder(res.Body).Decode(&body)
			if res.StatusCode != http.StatusBadRequest || !strings.Contains(body["error"], tt.message) {
				t.Errorf("estado %d, error %q; se esperaba 400 con %q", res.StatusCode, body["error"], tt.message)
			}
		})
	}

	res := upload(t, srv.URL, []byte("hola"))
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("un archivo inválido debería dar 400, dio %d", res.StatusCode)
	}
}

func TestPreview(t *testing.T) {
	srv := newServer(t)
	res, err := http.Get(srv.URL + "/api/preview?preset=nes&voice=3&pitch=38")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var head [4]byte
	res.Body.Read(head[:])
	if res.StatusCode != http.StatusOK || string(head[:]) != "RIFF" || res.Header.Get("Cache-Control") == "" {
		t.Errorf("estado %d, cabecera %q, caché %q", res.StatusCode, head, res.Header.Get("Cache-Control"))
	}
	for _, query := range []string{"preset=zx&voice=0&pitch=60", "preset=nes&voice=x&pitch=60", "preset=nes&voice=0&pitch=200"} {
		r, err := http.Get(srv.URL + "/api/preview?" + query)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: estado %d, se esperaba 400", query, r.StatusCode)
		}
	}
}
