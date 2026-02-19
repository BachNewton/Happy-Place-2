package render

import "happy-place-2/internal/maps"

// Connection bitmask constants for connected tiles (4-neighbor).
const (
	ConnN uint8 = 1
	ConnE uint8 = 2
	ConnS uint8 = 4
	ConnW uint8 = 8
)

// Blob bitmask constants for blob/autotile tiles (8-neighbor).
const (
	BlobN  uint8 = 1 << 0 // 1
	BlobNE uint8 = 1 << 1 // 2
	BlobE  uint8 = 1 << 2 // 4
	BlobSE uint8 = 1 << 3 // 8
	BlobS  uint8 = 1 << 4 // 16
	BlobSW uint8 = 1 << 5 // 32
	BlobW  uint8 = 1 << 6 // 64
	BlobNW uint8 = 1 << 7 // 128
)

// neighborMask computes a 4-bit bitmask of same-name cardinal neighbors.
func neighborMask(name string, wx, wy int, m *maps.Map) uint8 {
	if m == nil {
		return 0
	}
	var mask uint8
	if m.TileAt(wx, wy-1).Name == name {
		mask |= ConnN
	}
	if m.TileAt(wx+1, wy).Name == name {
		mask |= ConnE
	}
	if m.TileAt(wx, wy+1).Name == name {
		mask |= ConnS
	}
	if m.TileAt(wx-1, wy).Name == name {
		mask |= ConnW
	}
	return mask
}

// blobNeighborMask computes an 8-bit bitmask of same-name neighbors.
// Diagonals are only set if both adjacent cardinals also match,
// preventing diagonal "leaks" through cardinal gaps.
func blobNeighborMask(name string, wx, wy int, m *maps.Map) uint8 {
	if m == nil {
		return 0
	}
	var mask uint8

	n := m.TileAt(wx, wy-1).Name == name
	e := m.TileAt(wx+1, wy).Name == name
	s := m.TileAt(wx, wy+1).Name == name
	w := m.TileAt(wx-1, wy).Name == name

	if n {
		mask |= BlobN
	}
	if e {
		mask |= BlobE
	}
	if s {
		mask |= BlobS
	}
	if w {
		mask |= BlobW
	}

	// Diagonals only count if both adjacent cardinals are present
	if n && e && m.TileAt(wx+1, wy-1).Name == name {
		mask |= BlobNE
	}
	if s && e && m.TileAt(wx+1, wy+1).Name == name {
		mask |= BlobSE
	}
	if s && w && m.TileAt(wx-1, wy+1).Name == name {
		mask |= BlobSW
	}
	if n && w && m.TileAt(wx-1, wy-1).Name == name {
		mask |= BlobNW
	}

	return mask
}

// borderBlobMaskToParts maps a border blob mask to part names.
// The mask represents which neighbors ARE the border blob tile (e.g., path).
// For 1 cardinal, edge names are FLIPPED (placed on neighboring tile).
// For 2 adjacent cardinals, INNER corners are used (flipped) — at path bends,
// the grass tile should show mostly path with a small grass notch, not the outer
// corner sprite which is 93% grass.
// For 3 cardinals (1 missing), edge for missing direction (NOT flipped).
// For 4 cardinals with missing diagonals, inner corners (NOT flipped).
func borderBlobMaskToParts(mask uint8) []string {
	if mask == 0 {
		return nil
	}

	n := mask&BlobN != 0
	e := mask&BlobE != 0
	s := mask&BlobS != 0
	w := mask&BlobW != 0
	ne := mask&BlobNE != 0
	se := mask&BlobSE != 0
	sw := mask&BlobSW != 0
	nw := mask&BlobNW != 0

	cardinals := 0
	if n {
		cardinals++
	}
	if e {
		cardinals++
	}
	if s {
		cardinals++
	}
	if w {
		cardinals++
	}

	switch cardinals {
	case 1:
		// Single edge (flipped: path to S means this tile is at the N edge)
		if s {
			return []string{"edge_n"}
		}
		if n {
			return []string{"edge_s"}
		}
		if e {
			return []string{"edge_w"}
		}
		if w {
			return []string{"edge_e"}
		}

	case 2:
		// 2 adjacent = inner corner (flipped). Inner corners are ~60% path
		// which seamlessly connects to center tiles at path bends.
		// Outer corners (~6% path) would make the path nearly disappear.
		if s && e {
			return []string{"inner_nw"}
		}
		if s && w {
			return []string{"inner_ne"}
		}
		if n && e {
			return []string{"inner_sw"}
		}
		if n && w {
			return []string{"inner_se"}
		}
		// 2 opposite: no single sprite covers this
		return nil

	case 3:
		// Edge for the missing cardinal (NOT flipped)
		if !n {
			return []string{"edge_n"}
		}
		if !s {
			return []string{"edge_s"}
		}
		if !e {
			return []string{"edge_e"}
		}
		if !w {
			return []string{"edge_w"}
		}

	case 4:
		// All cardinals present — check diagonals for inner corners (NOT flipped)
		var parts []string
		if !nw {
			parts = append(parts, "inner_nw")
		}
		if !ne {
			parts = append(parts, "inner_ne")
		}
		if !sw {
			parts = append(parts, "inner_sw")
		}
		if !se {
			parts = append(parts, "inner_se")
		}
		if len(parts) == 0 {
			return []string{"center"}
		}
		return parts
	}

	return nil
}

// borderBlobOuterCorner checks if a tile has a diagonal-only border blob neighbor
// (no cardinal path neighbors). Returns the outer corner part name, or "".
// Used for the convex outer corners of path shapes where the diagonal grass tile
// needs a small rounding.
func borderBlobOuterCorner(name string, wx, wy int, m *maps.Map) string {
	if m == nil {
		return ""
	}

	// If any cardinal matches, this isn't a diagonal-only case
	if m.TileAt(wx, wy-1).Name == name || m.TileAt(wx+1, wy).Name == name ||
		m.TileAt(wx, wy+1).Name == name || m.TileAt(wx-1, wy).Name == name {
		return ""
	}

	// Check diagonals (without cardinal gating)
	se := m.TileAt(wx+1, wy+1).Name == name
	sw := m.TileAt(wx-1, wy+1).Name == name
	ne := m.TileAt(wx+1, wy-1).Name == name
	nw := m.TileAt(wx-1, wy-1).Name == name

	// Single diagonal → outer corner (sprite name matches the grass side)
	count := 0
	if se {
		count++
	}
	if sw {
		count++
	}
	if ne {
		count++
	}
	if nw {
		count++
	}

	if count == 1 {
		if se {
			return "outer_nw"
		}
		if sw {
			return "outer_ne"
		}
		if ne {
			return "outer_sw"
		}
		if nw {
			return "outer_se"
		}
	}

	return ""
}

// tileNameOrder defines the display order for tile types in the debug view.
// Matches the order from the original tileList.
var tileNameOrder = []string{
	"grass", "wall", "water", "tree", "path", "door", "floor",
	"fence", "flowers", "tall_grass",
}
