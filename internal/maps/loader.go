package maps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// colorNames maps color names from JSON to ANSI codes.
var colorNames = map[string]int{
	"black":          30,
	"red":            31,
	"green":          32,
	"yellow":         33,
	"blue":           34,
	"magenta":        35,
	"cyan":           36,
	"white":          37,
	"gray":           90,
	"grey":           90,
	"bright_red":     91,
	"bright_green":   92,
	"bright_yellow":  93,
	"bright_blue":    94,
	"bright_magenta": 95,
	"bright_cyan":    96,
	"bright_white":   97,
}

func resolveColor(name string) int {
	if code, ok := colorNames[name]; ok {
		return code
	}
	return 37
}

// TileDef defines the visual and gameplay properties of a tile type.
type TileDef struct {
	Char     rune
	Fg       int
	Bg       int
	Walkable bool
	Name     string
}

// Portal defines a teleport point linking two maps.
type Portal struct {
	X, Y              int
	TargetMap         string
	TargetX, TargetY  int
}

// Spawn defines the spawn point coordinates.
type Spawn struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Map represents a loaded tile map.
type Map struct {
	Name    string
	Width   int
	Height  int
	SpawnX  int
	SpawnY  int
	Tiles   [][]int   // [y][x] tile indices
	Legend  []TileDef // index → tile definition
	Portals []Portal
}

// jsonMap is the on-disk JSON format.
type jsonMap struct {
	Name    string             `json:"name"`
	Width   int                `json:"width"`
	Height  int                `json:"height"`
	Spawn   Spawn              `json:"spawn"`
	Tiles   [][]int            `json:"tiles"`
	Legend  map[string]jsonTile `json:"legend"`
	Portals []jsonPortal        `json:"portals,omitempty"`
}

type jsonPortal struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	TargetMap string `json:"target_map"`
	TargetX   int    `json:"target_x"`
	TargetY   int    `json:"target_y"`
}

type jsonTile struct {
	Char     string `json:"char"`
	Fg       string `json:"fg"`
	Bg       string `json:"bg,omitempty"`
	Walkable bool   `json:"walkable"`
	Name     string `json:"name"`
}

// LoadMap reads a JSON map file from disk.
func LoadMap(path string) (*Map, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read map file: %w", err)
	}

	var jm jsonMap
	if err := json.Unmarshal(data, &jm); err != nil {
		return nil, fmt.Errorf("parse map JSON: %w", err)
	}

	// Build legend array — find max index
	maxIdx := 0
	for k := range jm.Legend {
		var idx int
		fmt.Sscanf(k, "%d", &idx)
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	legend := make([]TileDef, maxIdx+1)
	for k, jt := range jm.Legend {
		var idx int
		fmt.Sscanf(k, "%d", &idx)
		ch := '?'
		if len(jt.Char) > 0 {
			ch = rune(jt.Char[0])
		}
		legend[idx] = TileDef{
			Char:     ch,
			Fg:       resolveColor(jt.Fg),
			Bg:       resolveColor(jt.Bg),
			Walkable: jt.Walkable,
			Name:     jt.Name,
		}
	}

	// Validate tile dimensions
	if len(jm.Tiles) != jm.Height {
		return nil, fmt.Errorf("tile rows %d != declared height %d", len(jm.Tiles), jm.Height)
	}
	for y, row := range jm.Tiles {
		if len(row) != jm.Width {
			return nil, fmt.Errorf("row %d has %d tiles, expected %d", y, len(row), jm.Width)
		}
	}

	portals := make([]Portal, len(jm.Portals))
	for i, jp := range jm.Portals {
		portals[i] = Portal{
			X: jp.X, Y: jp.Y,
			TargetMap: jp.TargetMap,
			TargetX: jp.TargetX, TargetY: jp.TargetY,
		}
	}

	return &Map{
		Name:    jm.Name,
		Width:   jm.Width,
		Height:  jm.Height,
		SpawnX:  jm.Spawn.X,
		SpawnY:  jm.Spawn.Y,
		Tiles:   jm.Tiles,
		Legend:  legend,
		Portals: portals,
	}, nil
}

// TileAt returns the tile definition at the given coordinates.
// Returns a default non-walkable tile for out-of-bounds coordinates.
func (m *Map) TileAt(x, y int) TileDef {
	if x < 0 || x >= m.Width || y < 0 || y >= m.Height {
		return TileDef{Char: ' ', Fg: 37, Walkable: false, Name: "void"}
	}
	idx := m.Tiles[y][x]
	if idx < 0 || idx >= len(m.Legend) {
		return TileDef{Char: '?', Fg: 37, Walkable: false, Name: "unknown"}
	}
	return m.Legend[idx]
}

// IsWalkable checks if the tile at x,y can be walked on.
func (m *Map) IsWalkable(x, y int) bool {
	return m.TileAt(x, y).Walkable
}

// PortalAt returns the portal at the given coordinates, or nil if none.
func (m *Map) PortalAt(x, y int) *Portal {
	for i := range m.Portals {
		if m.Portals[i].X == x && m.Portals[i].Y == y {
			return &m.Portals[i]
		}
	}
	return nil
}

// LoadMaps scans a directory for *.json files, loads each as a Map,
// and returns them indexed by Name. Validates portal target_map references.
func LoadMaps(dir string) (map[string]*Map, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read maps directory: %w", err)
	}

	allMaps := make(map[string]*Map)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		m, err := LoadMap(path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", entry.Name(), err)
		}
		if _, exists := allMaps[m.Name]; exists {
			return nil, fmt.Errorf("duplicate map name %q in %s", m.Name, entry.Name())
		}
		allMaps[m.Name] = m
	}

	// Validate portal references
	for name, m := range allMaps {
		for _, p := range m.Portals {
			if _, ok := allMaps[p.TargetMap]; !ok {
				return nil, fmt.Errorf("map %q portal at (%d,%d) references unknown map %q", name, p.X, p.Y, p.TargetMap)
			}
		}
	}

	return allMaps, nil
}

// DefaultMap returns a simple fallback map if no JSON file is available.
func DefaultMap() *Map {
	w, h := 60, 30
	tiles := make([][]int, h)
	for y := 0; y < h; y++ {
		tiles[y] = make([]int, w)
		for x := 0; x < w; x++ {
			if x == 0 || x == w-1 || y == 0 || y == h-1 {
				tiles[y][x] = 1 // wall
			} else {
				tiles[y][x] = 0 // grass
			}
		}
	}

	return &Map{
		Name:   "Default",
		Width:  w,
		Height: h,
		SpawnX: w / 2,
		SpawnY: h / 2,
		Tiles:  tiles,
		Legend: []TileDef{
			{Char: '.', Fg: 32, Walkable: true, Name: "grass"},
			{Char: '#', Fg: 90, Walkable: false, Name: "wall"},
		},
	}
}
