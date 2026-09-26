package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPackHandler(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"format":1}`), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "icons"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "icons", "gold.abc.png"), []byte("png"), 0o644)
	_ = os.WriteFile(filepath.Join(filepath.Dir(dir), "secret.txt"), []byte("no"), 0o644)
	h := packHandler(dir)
	cases := []struct {
		path, cache string
		code        int
	}{
		{"/v1/sprites/pack/manifest.json", "public, max-age=60", 200},
		{"/v1/sprites/pack/icons/gold.abc.png", "public, max-age=31536000, immutable", 200},
		{"/v1/sprites/pack/../secret.txt", "", 404},
		{"/v1/sprites/pack/icons/", "", 404},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.code {
			t.Errorf("%s: code %d, want %d", c.path, rec.Code, c.code)
		}
		if c.cache != "" && rec.Header().Get("Cache-Control") != c.cache {
			t.Errorf("%s: cache %q", c.path, rec.Header().Get("Cache-Control"))
		}
	}
}
