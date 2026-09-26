package sprite

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
)

// ErrMissing means the layer image does not exist for that animation.
var ErrMissing = errors.New("sprite image missing")

// Assets loads spritesheet PNGs from a local directory and, when a remote
// base URL is configured, lazily downloads missing files into that
// directory (so the full LPC art set never has to be vendored).
type Assets struct {
	Dir    string
	Remote string
	client *http.Client
	mem    *lru.Cache[string, *image.NRGBA]
	mu     sync.Mutex
	flight map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	img *image.NRGBA
	err error
}

func NewAssets(dir, remote string) *Assets {
	return &Assets{
		Dir: dir, Remote: strings.TrimRight(remote, "/"),
		client: &http.Client{Timeout: 20 * time.Second},
		mem:    lru.New[string, *image.NRGBA](768, 0),
		flight: map[string]*call{},
	}
}

// Image returns the decoded image at rel (relative to spritesheets/).
func (a *Assets) Image(ctx context.Context, rel string) (*image.NRGBA, error) {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "/") {
		return nil, fmt.Errorf("bad asset path %q", rel)
	}
	if img, ok := a.mem.Get(rel); ok {
		if img == nil {
			return nil, ErrMissing
		}
		return img, nil
	}
	a.mu.Lock()
	if c, ok := a.flight[rel]; ok {
		a.mu.Unlock()
		c.wg.Wait()
		return c.img, c.err
	}
	c := &call{}
	c.wg.Add(1)
	a.flight[rel] = c
	a.mu.Unlock()

	c.img, c.err = a.load(ctx, rel)
	if c.err == nil || errors.Is(c.err, ErrMissing) {
		a.mem.Put(rel, c.img)
	}
	c.wg.Done()
	a.mu.Lock()
	delete(a.flight, rel)
	a.mu.Unlock()
	return c.img, c.err
}

func (a *Assets) load(ctx context.Context, rel string) (*image.NRGBA, error) {
	local := filepath.Join(a.Dir, filepath.FromSlash(rel))
	if f, err := os.Open(local); err == nil {
		defer f.Close()
		return decode(f)
	}
	if st, err := os.Stat(local + ".missing"); err == nil && time.Since(st.ModTime()) < 24*time.Hour {
		return nil, ErrMissing
	}
	if a.Remote == "" {
		return nil, ErrMissing
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Remote+"/"+rel, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rel, err)
	}
	defer resp.Body.Close()
	_ = os.MkdirAll(filepath.Dir(local), 0o755)
	if resp.StatusCode == http.StatusNotFound {
		_ = os.WriteFile(local+".missing", nil, 0o644)
		return nil, ErrMissing
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", rel, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	tmp := local + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err == nil {
		_ = os.Rename(tmp, local)
	}
	return decode(strings.NewReader(string(data)))
}

func decode(r io.Reader) (*image.NRGBA, error) {
	img, err := png.Decode(r)
	if err != nil {
		return nil, err
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n, nil
	}
	b := img.Bounds()
	n := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(n, n.Bounds(), img, b.Min, draw.Src)
	return n, nil
}
