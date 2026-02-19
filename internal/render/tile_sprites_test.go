package render

import (
	"fmt"
	"strings"
	"testing"

	"happy-place-2/internal/maps"
)

// TestBorderBlobMaskToParts verifies the mapping from 8-bit neighbor mask
// to blob part names for the border blob system.
func TestBorderBlobMaskToParts(t *testing.T) {
	tests := []struct {
		name     string
		mask     uint8
		expected []string // nil means no transition
	}{
		// No neighbors
		{"no path neighbors", 0, nil},

		// 1 cardinal (flipped edges)
		{"path to S", BlobS, []string{"edge_n"}},
		{"path to N", BlobN, []string{"edge_s"}},
		{"path to E", BlobE, []string{"edge_w"}},
		{"path to W", BlobW, []string{"edge_e"}},

		// 2 adjacent cardinals (flipped inner corners — mostly path at bends)
		{"path to S+E", BlobS | BlobE, []string{"inner_nw"}},
		{"path to S+W", BlobS | BlobW, []string{"inner_ne"}},
		{"path to N+E", BlobN | BlobE, []string{"inner_sw"}},
		{"path to N+W", BlobN | BlobW, []string{"inner_se"}},

		// 2 adjacent + diagonal (should still be inner corner)
		{"path to S+E+SE", BlobS | BlobE | BlobSE, []string{"inner_nw"}},

		// 2 opposite cardinals (no valid sprite)
		{"path to N+S", BlobN | BlobS, nil},
		{"path to E+W", BlobE | BlobW, nil},

		// 3 cardinals (edge for missing direction, NOT flipped)
		{"path to S+E+W, missing N", BlobS | BlobE | BlobW, []string{"edge_n"}},
		{"path to N+E+W, missing S", BlobN | BlobE | BlobW, []string{"edge_s"}},
		{"path to N+S+W, missing E", BlobN | BlobS | BlobW, []string{"edge_e"}},
		{"path to N+S+E, missing W", BlobN | BlobS | BlobE, []string{"edge_w"}},

		// 4 cardinals, missing diagonals (inner corners, NOT flipped)
		{"all cardinals, missing NW", BlobN | BlobE | BlobS | BlobW | BlobNE | BlobSE | BlobSW, []string{"inner_nw"}},
		{"all cardinals, missing NE", BlobN | BlobE | BlobS | BlobW | BlobNW | BlobSE | BlobSW, []string{"inner_ne"}},
		{"all cardinals, missing SW", BlobN | BlobE | BlobS | BlobW | BlobNE | BlobNW | BlobSE, []string{"inner_sw"}},
		{"all cardinals, missing SE", BlobN | BlobE | BlobS | BlobW | BlobNE | BlobNW | BlobSW, []string{"inner_se"}},

		// 4 cardinals, missing 2 diagonals
		{"all cardinals, missing NW+SE", BlobN | BlobE | BlobS | BlobW | BlobNE | BlobSW, []string{"inner_nw", "inner_se"}},

		// 4 cardinals, all diagonals present → center
		{"all neighbors", 0xFF, []string{"center"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := borderBlobMaskToParts(tt.mask)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Errorf("expected %v, got nil", tt.expected)
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, got)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("expected %v, got %v", tt.expected, got)
					return
				}
			}
		})
	}
}

// buildTestMap creates a maps.Map from a 2D grid of tile names.
func buildTestMap(grid [][]string) *maps.Map {
	height := len(grid)
	width := len(grid[0])

	// Collect unique tile names and assign legend indices
	nameToIdx := map[string]int{}
	var legend []maps.TileDef
	for _, row := range grid {
		for _, name := range row {
			if _, ok := nameToIdx[name]; !ok {
				idx := len(legend)
				nameToIdx[name] = idx
				legend = append(legend, maps.TileDef{Name: name, Walkable: true})
			}
		}
	}

	tiles := make([][]int, height)
	for y := 0; y < height; y++ {
		tiles[y] = make([]int, width)
		for x := 0; x < width; x++ {
			tiles[y][x] = nameToIdx[grid[y][x]]
		}
	}

	return &maps.Map{
		Width:  width,
		Height: height,
		Tiles:  tiles,
		Legend: legend,
		Name:   "test",
	}
}

// TestBlobNeighborMaskForBorderBlob verifies that blobNeighborMask correctly
// computes the mask for a non-blob tile checking for blob neighbors.
func TestBlobNeighborMaskForBorderBlob(t *testing.T) {
	// Path pattern:
	//   G G G G G
	//   G P P P G
	//   G G P G G
	//   G G P G G
	//   G G G G G
	m := buildTestMap([][]string{
		{"grass", "grass", "grass", "grass", "grass"},
		{"grass", "path", "path", "path", "grass"},
		{"grass", "grass", "path", "grass", "grass"},
		{"grass", "grass", "path", "grass", "grass"},
		{"grass", "grass", "grass", "grass", "grass"},
	})

	tests := []struct {
		name     string
		wx, wy   int
		expected uint8
	}{
		// No path neighbors
		{"(0,0) corner", 0, 0, 0},
		{"(4,4) corner", 4, 4, 0},

		// 1 cardinal
		{"(1,0) path to S", 1, 0, BlobS},
		{"(2,0) path to S", 2, 0, BlobS},
		{"(0,1) path to E", 0, 1, BlobE},
		{"(4,1) path to W", 4, 1, BlobW},
		{"(2,4) path to N", 2, 4, BlobN},

		// 2 adjacent at bend
		{"(1,2) path N+E", 1, 2, BlobN | BlobE | BlobNE},
		// (1,2): N=(1,1)=path, E=(2,2)=path, NE check: N&&E → (2,1)=path → NE set

		// Edge next to bend
		{"(3,1) path W only", 3, 1, BlobW},
		// (3,1): W=(2,1)=path. N=(3,0)=grass, S=(3,2)=grass, E=(4,1)=grass

		// Corner at bend
		{"(3,2) path N+W", 3, 2, BlobN | BlobW | BlobNW},
		// (3,2): N=(3,1)=path, W=(2,2)=path, NW check: N&&W → (2,1)=path → NW set

		// Bottom of vertical path
		{"(1,3) path E only", 1, 3, BlobE},
		{"(3,3) path W only", 3, 3, BlobW},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := blobNeighborMask("path", tt.wx, tt.wy, m)
			if got != tt.expected {
				t.Errorf("blobNeighborMask(\"path\", %d, %d) = %s, want %s",
					tt.wx, tt.wy, formatMask(got), formatMask(tt.expected))
			}
		})
	}
}

// TestBorderBlobFullPattern prints which sprite each tile in a pattern would use.
func TestBorderBlobFullPattern(t *testing.T) {
	// L-shaped path
	m := buildTestMap([][]string{
		{"grass", "grass", "grass", "grass", "grass", "grass", "grass"},
		{"grass", "grass", "path", "path", "path", "grass", "grass"},
		{"grass", "grass", "grass", "grass", "path", "grass", "grass"},
		{"grass", "grass", "grass", "grass", "path", "grass", "grass"},
		{"grass", "grass", "grass", "grass", "grass", "grass", "grass"},
	})

	t.Log("Map layout (P=path, .=grass):")
	for y := 0; y < m.Height; y++ {
		var row strings.Builder
		for x := 0; x < m.Width; x++ {
			if m.TileAt(x, y).Name == "path" {
				row.WriteString("  P  ")
			} else {
				row.WriteString("  .  ")
			}
		}
		t.Log(row.String())
	}

	t.Log("")
	t.Log("Border blob sprite assignments (grass tiles):")
	for y := 0; y < m.Height; y++ {
		var row strings.Builder
		for x := 0; x < m.Width; x++ {
			tile := m.TileAt(x, y)
			if tile.Name == "path" {
				row.WriteString(fmt.Sprintf(" %-14s", "[center]"))
			} else {
				label := borderBlobLabel("path", x, y, m)
				row.WriteString(fmt.Sprintf(" %-14s", label))
			}
		}
		t.Log(row.String())
	}

	// Also test a plus-shaped path
	t.Log("")
	t.Log("=== Plus-shaped path ===")
	m2 := buildTestMap([][]string{
		{"grass", "grass", "grass", "grass", "grass"},
		{"grass", "grass", "path", "grass", "grass"},
		{"grass", "path", "path", "path", "grass"},
		{"grass", "grass", "path", "grass", "grass"},
		{"grass", "grass", "grass", "grass", "grass"},
	})

	t.Log("Map layout:")
	for y := 0; y < m2.Height; y++ {
		var row strings.Builder
		for x := 0; x < m2.Width; x++ {
			if m2.TileAt(x, y).Name == "path" {
				row.WriteString("  P  ")
			} else {
				row.WriteString("  .  ")
			}
		}
		t.Log(row.String())
	}

	t.Log("")
	t.Log("Sprite assignments:")
	for y := 0; y < m2.Height; y++ {
		var row strings.Builder
		for x := 0; x < m2.Width; x++ {
			tile := m2.TileAt(x, y)
			if tile.Name == "path" {
				row.WriteString(fmt.Sprintf(" %-14s", "[center]"))
			} else {
				label := borderBlobLabel("path", x, y, m2)
				row.WriteString(fmt.Sprintf(" %-14s", label))
			}
		}
		t.Log(row.String())
	}
}

// borderBlobLabel returns the sprite label for a grass tile in test output.
func borderBlobLabel(bbName string, x, y int, m *maps.Map) string {
	mask := blobNeighborMask(bbName, x, y, m)
	if mask != 0 {
		parts := borderBlobMaskToParts(mask)
		if parts == nil {
			return fmt.Sprintf("NONE(%s)", formatMask(mask))
		}
		return strings.Join(parts, "+")
	}
	if part := borderBlobOuterCorner(bbName, x, y, m); part != "" {
		return part
	}
	return "."
}

// TestBorderBlobOuterCorner verifies diagonal-only detection for outer corners.
func TestBorderBlobOuterCorner(t *testing.T) {
	// Single isolated path tile — diagonal grass tiles have NO cardinal path neighbors
	m := buildTestMap([][]string{
		{"grass", "grass", "grass"},
		{"grass", "path", "grass"},
		{"grass", "grass", "grass"},
	})

	tests := []struct {
		name     string
		wx, wy   int
		expected string
	}{
		// Diagonal-only corners
		{"(0,0) path at SE", 0, 0, "outer_nw"},
		{"(2,0) path at SW", 2, 0, "outer_ne"},
		{"(0,2) path at NE", 0, 2, "outer_sw"},
		{"(2,2) path at NW", 2, 2, "outer_se"},

		// Cardinal neighbors → not a diagonal-only case
		{"(1,0) has cardinal S", 1, 0, ""},
		{"(0,1) has cardinal E", 0, 1, ""},
		{"(2,1) has cardinal W", 2, 1, ""},
		{"(1,2) has cardinal N", 1, 2, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := borderBlobOuterCorner("path", tt.wx, tt.wy, m)
			if got != tt.expected {
				t.Errorf("borderBlobOuterCorner(\"path\", %d, %d) = %q, want %q",
					tt.wx, tt.wy, got, tt.expected)
			}
		})
	}
}

// formatMask returns a human-readable string for an 8-bit blob mask.
func formatMask(mask uint8) string {
	if mask == 0 {
		return "0"
	}
	var parts []string
	if mask&BlobN != 0 {
		parts = append(parts, "N")
	}
	if mask&BlobNE != 0 {
		parts = append(parts, "NE")
	}
	if mask&BlobE != 0 {
		parts = append(parts, "E")
	}
	if mask&BlobSE != 0 {
		parts = append(parts, "SE")
	}
	if mask&BlobS != 0 {
		parts = append(parts, "S")
	}
	if mask&BlobSW != 0 {
		parts = append(parts, "SW")
	}
	if mask&BlobW != 0 {
		parts = append(parts, "W")
	}
	if mask&BlobNW != 0 {
		parts = append(parts, "NW")
	}
	return strings.Join(parts, "|")
}
