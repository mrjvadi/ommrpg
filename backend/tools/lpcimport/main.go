// Command lpcimport converts the Universal LPC Spritesheet Character
// Generator's sheet_definitions and palette_definitions into the catalog
// format used by sprite-service.
//
//	git clone --depth 1 https://github.com/liberatedpixelcup/Universal-LPC-Spritesheet-Character-Generator lpc
//	go run ./tools/lpcimport -repo lpc -out ../assets/lpc/catalog.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mrjvadi/ommrpg/backend/pkg/sprite"
)

// Only the classic LPC animation set is composed (fixed universal layout).
var animations = []string{"spellcast", "thrust", "walk", "slash", "shoot", "hurt"}

var bodyTypes = []string{"male", "female", "teen", "muscular", "pregnant", "child"}

func main() {
	repo := flag.String("repo", "", "path to a checkout of the LPC generator repository")
	out := flag.String("out", "catalog.json", "output catalog file")
	flag.Parse()
	if *repo == "" {
		log.Fatal("-repo is required")
	}
	cat := &sprite.Catalog{
		Source: sprite.CatalogSource{
			Name:    "Universal LPC Spritesheet Character Generator",
			Repo:    "https://github.com/liberatedpixelcup/Universal-LPC-Spritesheet-Character-Generator",
			Commit:  gitHead(*repo),
			License: "Artwork is CC-BY-SA / CC-BY / OGA-BY / GPL / CC0 per item; credit every author listed in item credits.",
		},
		BodyTypes:  bodyTypes,
		Animations: animations,
		Materials:  map[string]*sprite.Material{},
		Items:      map[string]*sprite.Item{},
	}
	loadPalettes(filepath.Join(*repo, "palette_definitions"), cat)
	defs := filepath.Join(*repo, "sheet_definitions")
	err := filepath.Walk(defs, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") || strings.HasPrefix(info.Name(), "meta_") {
			return err
		}
		it, err := convert(defs, path)
		if err != nil {
			log.Printf("skip %s: %v", path, err)
			return nil
		}
		if it != nil {
			if _, dup := cat.Items[it.ID]; dup {
				log.Printf("duplicate id %s (%s)", it.ID, path)
			}
			cat.Items[it.ID] = it
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	raw, err := json.Marshal(cat)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s: %d items, %d materials (%d bytes)\n", *out, len(cat.Items), len(cat.Materials), len(raw))
}

func gitHead(repo string) string {
	b, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

func loadPalettes(dir string, cat *sprite.Catalog) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("palettes: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		mat := e.Name()
		var meta struct {
			Default string `json:"default"`
			Base    string `json:"base"`
		}
		readJSON(filepath.Join(dir, mat, "meta_"+mat+".json"), &meta)
		m := &sprite.Material{Default: meta.Default, Base: meta.Base, Palettes: map[string]map[string][]string{}}
		files, _ := filepath.Glob(filepath.Join(dir, mat, mat+"_*.json"))
		for _, f := range files {
			version := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), mat+"_"), ".json")
			var p map[string][]string
			readJSON(f, &p)
			m.Palettes[version] = p
		}
		cat.Materials[mat] = m
	}
}

func readJSON(path string, v any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("%s: %v", path, err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		log.Fatalf("%s: %v", path, err)
	}
}

var layerKey = regexp.MustCompile(`^layer_\d+$`)

func convert(root, path string) (*sprite.Item, error) {
	var d map[string]json.RawMessage
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(root, path)
	it := &sprite.Item{
		ID:       strings.TrimSuffix(filepath.Base(path), ".json"),
		Category: filepath.ToSlash(filepath.Dir(rel)),
	}
	str := func(k string) string {
		var s string
		_ = json.Unmarshal(d[k], &s)
		return s
	}
	it.Name, it.TypeName = str("name"), str("type_name")
	_ = json.Unmarshal(d["priority"], &it.Priority)
	_ = json.Unmarshal(d["variants"], &it.Variants)
	_ = json.Unmarshal(d["match_body_color"], &it.MatchBodyColor)
	_ = json.Unmarshal(d["animations"], &it.Animations)
	_ = json.Unmarshal(d["tags"], &it.Tags)
	_ = json.Unmarshal(d["credits"], &it.Credits)
	keys := make([]string, 0)
	for k := range d {
		if layerKey.MatchString(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		var l map[string]json.RawMessage
		if err := json.Unmarshal(d[k], &l); err != nil {
			continue
		}
		if _, custom := l["custom_animation"]; custom {
			continue // oversized animations are not part of the universal sheet
		}
		layer := sprite.ItemLayer{Paths: map[string]string{}}
		_ = json.Unmarshal(l["zPos"], &layer.Z)
		for _, bt := range bodyTypes {
			var p string
			if json.Unmarshal(l[bt], &p) == nil && p != "" {
				layer.Paths[bt] = p
			}
		}
		if len(layer.Paths) > 0 {
			it.Layers = append(it.Layers, layer)
		}
	}
	if len(it.Layers) == 0 {
		return nil, fmt.Errorf("no drawable layers")
	}
	it.Recolors = recolors(d["recolors"], it.TypeName)
	return it, nil
}

func recolors(raw json.RawMessage, typeName string) []sprite.Recolor {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	parse := func(r json.RawMessage) sprite.Recolor {
		var x struct {
			TypeName string          `json:"type_name"`
			Material string          `json:"material"`
			Base     string          `json:"base"`
			Source   json.RawMessage `json:"source"`
			Label    string          `json:"label"`
		}
		_ = json.Unmarshal(r, &x)
		rc := sprite.Recolor{TypeName: x.TypeName, Material: x.Material, Base: x.Base, Label: x.Label}
		if rc.TypeName == "" {
			rc.TypeName = typeName
		}
		var src []string
		if json.Unmarshal(x.Source, &src) == nil {
			rc.Source = src
		}
		return rc
	}
	if _, single := m["material"]; single {
		return []sprite.Recolor{parse(raw)}
	}
	var keys []string
	for k := range m {
		if strings.HasPrefix(k, "color_") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []sprite.Recolor
	for _, k := range keys {
		out = append(out, parse(m[k]))
	}
	return out
}
