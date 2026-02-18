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

// tileNameOrder defines the display order for tile types in the debug view.
// Matches the order from the original tileList.
var tileNameOrder = []string{
	"grass", "wall", "water", "tree", "path", "door", "floor",
	"fence", "flowers", "tall_grass",
}
