// Package server exposes the catalog as a JSON API and a web UI.
package server

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/udaykishore-resu/kickhub/internal/catalog"
	"github.com/udaykishore-resu/kickhub/internal/engine"
)

//go:embed ui/index.html
var indexHTML []byte

// New returns the HTTP handler for the hub.
func New(c *catalog.Catalog) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	mux.HandleFunc("GET /api/templates", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, c.Search(r.URL.Query().Get("q"), r.URL.Query().Get("target")))
	})

	// Preview: the file tree and contents, without creating anything.
	mux.HandleFunc("POST /api/preview", func(w http.ResponseWriter, r *http.Request) {
		res, ok := render(c, w, r)
		if !ok {
			return
		}
		type file struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		files := []file{}
		for _, p := range res.Paths() {
			files = append(files, file{p, string(res.Files[p])})
		}
		checks, _ := engine.Doctor(res.Template, res.Target)
		writeJSON(w, http.StatusOK, map[string]any{"files": files, "requires": checks})
	})

	mux.HandleFunc("POST /api/generate", func(w http.ResponseWriter, r *http.Request) {
		res, ok := render(c, w, r)
		if !ok {
			return
		}
		var buf bytes.Buffer
		if err := res.Zip(&buf); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, res.Name))
		w.Write(buf.Bytes())
	})

	return mux
}

func render(c *catalog.Catalog, w http.ResponseWriter, r *http.Request) (*engine.Result, bool) {
	var req engine.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return nil, false
	}
	res, err := engine.Render(c, req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return nil, false
	}
	return res, true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
