package render

import "happy-place-2/internal/maps"

// Connection bitmask constants for connected tiles.
const (
	ConnN uint8 = 1
	ConnE uint8 = 2
	ConnS uint8 = 4
	ConnW uint8 = 8
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

// tileNameOrder defines the display order for tile types in the debug view.
// Matches the order from the original tileList.
var tileNameOrder = []string{
	"grass", "wall", "water", "tree", "path", "door", "floor",
	"fence", "flowers", "tall_grass",
}
