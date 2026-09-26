// Package sprite composes layered pixel-art sprite sheets, in the spirit of
// the Universal LPC Spritesheet Character Generator, and procedurally draws
// creatures, item icons and tiles from seeds.
package sprite

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Catalog is the importable description of all layered parts. It is
// generated from the LPC generator's sheet_definitions by tools/lpcimport.
type Catalog struct {
	Source     CatalogSource        `json:"source"`
	BodyTypes  []string             `json:"body_types"`
	Animations []string             `json:"animations"`
	Materials  map[string]*Material `json:"materials"`
	Items      map[string]*Item     `json:"items"`
}

type CatalogSource struct {
	Name    string `json:"name"`
	Repo    string `json:"repo"`
	Commit  string `json:"commit"`
	License string `json:"license_note"`
}

// Material is a family of palettes (hair, body, cloth, metal...).
type Material struct {
	Default  string                         `json:"default"`
	Base     string                         `json:"base"`
	Palettes map[string]map[string][]string `json:"palettes"` // version -> color name -> hex list
}

// Recolor declares one recolourable channel of an item.
type Recolor struct {
	TypeName string   `json:"type_name"`
	Material string   `json:"material"`
	Base     string   `json:"base,omitempty"`
	Source   []string `json:"source,omitempty"`
	Label    string   `json:"label,omitempty"`
}

// ItemLayer is one drawing layer of an item.
type ItemLayer struct {
	Z     int               `json:"z"`
	Paths map[string]string `json:"paths"` // body type -> spritesheets-relative dir
}

type Credit struct {
	File     string   `json:"file"`
	Authors  []string `json:"authors"`
	Licenses []string `json:"licenses"`
	URLs     []string `json:"urls"`
	Notes    string   `json:"notes,omitempty"`
}

// Item is one selectable part (a hair style, a shirt, a sword...).
type Item struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Category       string      `json:"category"`
	TypeName       string      `json:"type_name"`
	Priority       int         `json:"priority,omitempty"`
	Layers         []ItemLayer `json:"layers"`
	Variants       []string    `json:"variants,omitempty"`
	Recolors       []Recolor   `json:"recolors,omitempty"`
	MatchBodyColor bool        `json:"match_body_color,omitempty"`
	Animations     []string    `json:"animations,omitempty"`
	Tags           []string    `json:"tags,omitempty"`
	Credits        []Credit    `json:"credits,omitempty"`
}

// Supports reports whether the item can be drawn on a body type.
func (it *Item) Supports(bodyType string) bool {
	for _, l := range it.Layers {
		if _, ok := l.Paths[bodyType]; ok {
			return true
		}
	}
	return false
}

// HasAnimation reports whether an animation is declared for the item.
func (it *Item) HasAnimation(a string) bool {
	if len(it.Animations) == 0 {
		return true
	}
	for _, x := range it.Animations {
		if x == a {
			return true
		}
	}
	return false
}

// ColorOptions lists the colour names usable for recolor channel i.
func (c *Catalog) ColorOptions(it *Item, i int) []string {
	if i >= len(it.Recolors) {
		return nil
	}
	m := c.Materials[it.Recolors[i].Material]
	if m == nil {
		return nil
	}
	var out []string
	for name := range m.Palettes[m.Default] {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// LoadCatalog reads a catalog JSON file.
func LoadCatalog(path string) (*Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("catalog %s: %w", path, err)
	}
	for id, it := range c.Items {
		it.ID = id
	}
	return &c, nil
}

// resolvePalette returns the colours for a target key: "name",
// "version.name" or "material.version.name".
func (c *Catalog) resolvePalette(material, key string) ([]string, bool) {
	m := c.Materials[material]
	parts := strings.Split(key, ".")
	switch len(parts) {
	case 1:
		if m == nil {
			return nil, false
		}
		// default version first, then any other version
		if cols, ok := m.Palettes[m.Default][key]; ok {
			return cols, true
		}
		for _, v := range sortedKeys(m.Palettes) {
			if cols, ok := m.Palettes[v][key]; ok {
				return cols, true
			}
		}
	case 2:
		if m != nil {
			if cols, ok := m.Palettes[parts[0]][parts[1]]; ok {
				return cols, true
			}
		}
	case 3:
		if m2 := c.Materials[parts[0]]; m2 != nil {
			if cols, ok := m2.Palettes[parts[1]][parts[2]]; ok {
				return cols, true
			}
		}
	}
	return nil, false
}

// sourcePalette returns the base colours a recolor channel's image uses.
func (c *Catalog) sourcePalette(r Recolor) ([]string, bool) {
	if len(r.Source) > 0 {
		return r.Source, true
	}
	m := c.Materials[r.Material]
	if m == nil {
		return nil, false
	}
	if r.Base != "" {
		return c.resolvePalette(r.Material, r.Base)
	}
	cols, ok := m.Palettes[m.Default][m.Base]
	return cols, ok
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// DefaultRemote is the raw spritesheets URL of the catalog's pinned commit.
func DefaultRemote(c *Catalog) string {
	return "https://raw.githubusercontent.com/liberatedpixelcup/Universal-LPC-Spritesheet-Character-Generator/" + c.Source.Commit + "/spritesheets"
}
