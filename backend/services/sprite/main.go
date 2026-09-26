// sprite-service is the game's asset generator, modelled on the Universal LPC
// Spritesheet Character Generator: it composes layered, palette-recoloured
// character sheets from a catalog, and procedurally draws creatures, item
// icons and the world tileset from seeds. Everything is deterministic and
// cached, so URLs are immutable and CDN-friendly.
package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mrjvadi/ommrpg/backend/pkg/apperr"
	"github.com/mrjvadi/ommrpg/backend/pkg/bus"
	"github.com/mrjvadi/ommrpg/backend/pkg/config"
	c "github.com/mrjvadi/ommrpg/backend/pkg/contracts"
	"github.com/mrjvadi/ommrpg/backend/pkg/httpx"
	"github.com/mrjvadi/ommrpg/backend/pkg/lru"
	"github.com/mrjvadi/ommrpg/backend/pkg/sprite"
	"github.com/mrjvadi/ommrpg/backend/pkg/svc"
)

func main() {
	s := svc.New("sprite")
	cat, err := sprite.LoadCatalog(config.String("SPRITE_CATALOG", "../assets/lpc/catalog.json"))
	if err != nil {
		s.Fatal("catalog", err)
	}
	remote := config.String("SPRITE_REMOTE", "default")
	if remote == "default" {
		remote = sprite.DefaultRemote(cat)
	} else if remote == "off" {
		remote = ""
	}
	cacheDir := config.String("SPRITE_CACHE_DIR", "../.data/sprite-cache")
	a := &app{
		comp:  &sprite.Composer{Cat: cat, Assets: sprite.NewAssets(config.String("SPRITE_ASSET_DIR", filepath.Join(cacheDir, "spritesheets")), remote)},
		cache: lru.New[string, []byte](256, 0),
		dir:   filepath.Join(cacheDir, "renders"),
		sem:   make(chan struct{}, config.Int("SPRITE_RENDER_CONCURRENCY", 4)),
	}
	_ = os.MkdirAll(a.dir, 0o755)
	b, err := bus.Connect(s.Cfg.NATSURL, "sprite", s.Log)
	if err != nil {
		s.Fatal("nats", err)
	}
	defer b.Close()
	if err := bus.Handle(b, c.SpriteRandom, a.random); err != nil {
		s.Fatal("subscribe", err)
	}
	if err := bus.Handle(b, c.SpriteValidate, a.validate); err != nil {
		s.Fatal("subscribe", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/sprites/catalog", a.catalog)
	mux.HandleFunc("GET /v1/sprites/layout", a.layout)
	mux.HandleFunc("GET /v1/sprites/random", a.randomHTTP)
	mux.HandleFunc("GET /v1/sprites/character.png", a.characterPNG)
	mux.HandleFunc("POST /v1/sprites/character", a.characterPost)
	mux.HandleFunc("GET /v1/sprites/credits", a.credits)
	mux.HandleFunc("GET /v1/sprites/creature.png", a.creature)
	mux.HandleFunc("GET /v1/sprites/icon.png", a.icon)
	mux.HandleFunc("GET /v1/sprites/tileset.png", a.tilesetPNG)
	mux.HandleFunc("GET /v1/sprites/tileset.json", a.tilesetJSON)
	srv := &http.Server{
		Addr:              config.String("HTTP_ADDR", ":8090"),
		Handler:           httpx.Middleware(mux, config.List("CORS_ORIGINS", nil)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	s.Ready()
	if err := httpx.Serve(s.Ctx, srv); err != nil && err != http.ErrServerClosed {
		s.Fatal("http", err)
	}
}

type app struct {
	comp  *sprite.Composer
	cache *lru.Cache[string, []byte]
	dir   string
	sem   chan struct{}
}

// ---- NATS

func (a *app) random(_ context.Context, req c.SpriteRandomReq) (c.Recipe, error) {
	return a.comp.Cat.RandomRecipe(req.Seed, req.BodyType), nil
}

func (a *app) validate(_ context.Context, req c.Recipe) (c.SpriteValidateResp, error) {
	r, dropped := a.comp.Normalize(req)
	return c.SpriteValidateResp{Recipe: r, Dropped: dropped}, nil
}

// ---- HTTP

type catalogItem struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Category string     `json:"category"`
	Type     string     `json:"type"`
	Bodies   []string   `json:"bodies"`
	Variants []string   `json:"variants,omitempty"`
	Colors   [][]string `json:"colors,omitempty"` // per recolor channel
	Channels []string   `json:"channels,omitempty"`
}

func (a *app) catalog(w http.ResponseWriter, r *http.Request) {
	cat := a.comp.Cat
	out := struct {
		BodyTypes []string             `json:"body_types"`
		Layout    []sprite.AnimLayout  `json:"layout"`
		Items     []catalogItem        `json:"items"`
		Source    sprite.CatalogSource `json:"source"`
	}{BodyTypes: cat.BodyTypes, Layout: sprite.Layout, Source: cat.Source}
	for id, it := range cat.Items {
		ci := catalogItem{ID: id, Name: it.Name, Category: it.Category, Type: it.TypeName, Variants: it.Variants}
		for _, bt := range cat.BodyTypes {
			if it.Supports(bt) {
				ci.Bodies = append(ci.Bodies, bt)
			}
		}
		for i, rc := range it.Recolors {
			ci.Channels = append(ci.Channels, rc.TypeName)
			ci.Colors = append(ci.Colors, cat.ColorOptions(it, i))
		}
		out.Items = append(out.Items, ci)
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	httpx.JSON(w, 200, out)
}

func (a *app) layout(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, 200, map[string]any{"frame": sprite.Frame, "width": sprite.SheetW, "height": sprite.SheetH, "animations": sprite.Layout,
		"directions": []string{"up", "left", "down", "right"}})
}

func (a *app) randomHTTP(w http.ResponseWriter, r *http.Request) {
	s, _ := strconv.ParseUint(r.URL.Query().Get("seed"), 10, 64)
	rec := a.comp.Cat.RandomRecipe(s, r.URL.Query().Get("body"))
	httpx.JSON(w, 200, map[string]any{"recipe": rec, "url": characterURL(rec)})
}

// EncodeRecipe is the URL form of a recipe: base64url(JSON).
func EncodeRecipe(rec c.Recipe) string {
	raw, _ := json.Marshal(rec)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func characterURL(rec c.Recipe) string { return "/v1/sprites/character.png?r=" + EncodeRecipe(rec) }

func decodeRecipe(s string) (c.Recipe, error) {
	var rec c.Recipe
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil || len(raw) > 8<<10 {
		return rec, apperr.New(apperr.Invalid, "bad recipe encoding")
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return rec, apperr.New(apperr.Invalid, "bad recipe json")
	}
	return rec, nil
}

func (a *app) render(ctx context.Context, rec c.Recipe) (string, []byte, error) {
	rec, _ = a.comp.Normalize(rec)
	h := a.comp.Hash(rec)
	if b, ok := a.cache.Get(h); ok {
		return h, b, nil
	}
	path := filepath.Join(a.dir, h+".png")
	if b, err := os.ReadFile(path); err == nil {
		a.cache.Put(h, b)
		return h, b, nil
	}
	select {
	case a.sem <- struct{}{}:
		defer func() { <-a.sem }()
	case <-ctx.Done():
		return "", nil, ctx.Err()
	}
	b, err := a.comp.RenderPNG(ctx, rec)
	if err != nil {
		return "", nil, apperr.New(apperr.Unavailable, "render failed: %v", err)
	}
	a.cache.Put(h, b)
	_ = os.WriteFile(path, b, 0o644)
	return h, b, nil
}

func writePNG(w http.ResponseWriter, r *http.Request, etag string, b []byte) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", `"`+etag+`"`)
	if r.Header.Get("If-None-Match") == `"`+etag+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = w.Write(b)
}

func (a *app) characterPNG(w http.ResponseWriter, r *http.Request) {
	rec, err := decodeRecipe(r.URL.Query().Get("r"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	h, b, err := a.render(r.Context(), rec)
	if err != nil {
		httpx.Error(w, err)
		return
	}
	writePNG(w, r, h, b)
}

func (a *app) characterPost(w http.ResponseWriter, r *http.Request) {
	var rec c.Recipe
	if err := httpx.Decode(r, &rec); err != nil {
		httpx.Error(w, err)
		return
	}
	norm, dropped := a.comp.Normalize(rec)
	httpx.JSON(w, 200, map[string]any{
		"recipe": norm, "dropped": dropped, "hash": a.comp.Hash(norm),
		"url": characterURL(norm), "layout": sprite.Layout, "credits": a.comp.Credits(norm),
	})
}

func (a *app) credits(w http.ResponseWriter, r *http.Request) {
	rec, err := decodeRecipe(r.URL.Query().Get("r"))
	if err != nil {
		httpx.Error(w, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"credits": a.comp.Credits(rec), "source": a.comp.Cat.Source})
}

func qf(r *http.Request, k string) float64 {
	v, _ := strconv.ParseFloat(r.URL.Query().Get(k), 64)
	return v
}

func qu(r *http.Request, k string) uint64 {
	v, _ := strconv.ParseUint(r.URL.Query().Get(k), 10, 64)
	return v
}

func (a *app) cachedImage(w http.ResponseWriter, r *http.Request, key string, draw func() *image.NRGBA) {
	sum := sha1.Sum([]byte(key))
	etag := hex.EncodeToString(sum[:8])
	if b, ok := a.cache.Get(key); ok {
		writePNG(w, r, etag, b)
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, draw()); err != nil {
		httpx.Error(w, err)
		return
	}
	a.cache.Put(key, buf.Bytes())
	writePNG(w, r, etag, buf.Bytes())
}

func (a *app) creature(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	key := "creature:" + q.Encode()
	a.cachedImage(w, r, key, func() *image.NRGBA {
		return sprite.Creature(qu(r, "seed"), q.Get("family"), qf(r, "hue"), qf(r, "hue2"), q.Get("big") == "1" || q.Get("big") == "true")
	})
}

func (a *app) icon(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rar, _ := strconv.Atoi(q.Get("rarity"))
	a.cachedImage(w, r, "icon:"+q.Encode(), func() *image.NRGBA {
		return sprite.Icon(qu(r, "seed"), q.Get("shape"), qf(r, "hue"), rar)
	})
}

func (a *app) tilesetPNG(w http.ResponseWriter, r *http.Request) {
	a.cachedImage(w, r, "tileset:"+r.URL.Query().Get("seed"), func() *image.NRGBA {
		img, _ := sprite.Tileset(qu(r, "seed"))
		return img
	})
}

func (a *app) tilesetJSON(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, 200, sprite.TilesetInfo())
}
