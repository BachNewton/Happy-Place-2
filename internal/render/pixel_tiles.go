package render

import "happy-place-2/internal/maps"

// PixelTileSprite returns the pixel sprites for a tile at world position (wx,wy) at the given tick.
// For connected tiles, it computes the neighbor mask.
func PixelTileSprite(reg *SpriteRegistry, tile maps.TileDef, wx, wy int, tick uint64, m *maps.Map) PixelTileSprites {
	name := tile.Name

	if !reg.HasTile(name) {
		// Fallback: solid color from ANSI palette
		fgR, fgG, fgB := AnsiToRGB(tile.Fg)
		return PixelTileSprites{Base: FillPixelSprite(fgR, fgG, fgB)}
	}

	// Border blob self: render as plain center fill
	if reg.TileIsBorderBlob(name) {
		return PixelTileSprites{Base: reg.GetBlobTileSprite(name, 0xFF)}
	}

	if reg.TileIsBlob(name) {
		mask := blobNeighborMask(name, wx, wy, m)
		return PixelTileSprites{Base: reg.GetBlobTileSprite(name, mask)}
	}

	// Check if this tile neighbors a border blob tile (cardinal neighbors)
	for _, bbName := range reg.BorderBlobNames() {
		mask := blobNeighborMask(bbName, wx, wy, m)
		if mask != 0 {
			return PixelTileSprites{Base: reg.GetBorderBlobTileSprite(bbName, mask)}
		}
		// Diagonal-only: outer corner rounding at path convex corners
		if part := borderBlobOuterCorner(bbName, wx, wy, m); part != "" {
			if sprite, ok := reg.GetBlobPartSprite(bbName, part); ok {
				return PixelTileSprites{Base: sprite}
			}
		}
	}

	if reg.TileIsConnected(name) {
		mask := neighborMask(name, wx, wy, m)
		return PixelTileSprites{Base: reg.GetConnectedTileSprite(name, mask)}
	}

	return reg.GetTileSprites(name, tick)
}
