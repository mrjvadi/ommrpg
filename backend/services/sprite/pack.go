package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// packHandler serves the art pack rendered with Blender (tools/blender,
// published to assets/pack): world props as a pixel-art atlas and the UI
// icons. Every file except manifest.json has its content hash in its name,
// so it can be cached forever; the manifest is re-checked by clients.
func packHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.StripPrefix("/v1/sprites/pack", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		// files only: no directory listings, nothing outside the pack
		if st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean))); strings.Contains(clean, "..") || err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		if clean == "/manifest.json" {
			w.Header().Set("Cache-Control", "public, max-age=60")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		r.URL.Path = clean
		files.ServeHTTP(w, r)
	}))
}
