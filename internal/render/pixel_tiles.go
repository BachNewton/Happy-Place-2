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

	if reg.TileIsBlob(name) {
		mask := blobNeighborMask(name, wx, wy, m)
		return PixelTileSprites{Base: reg.GetBlobTileSprite(name, mask)}
	}

	if reg.TileIsConnected(name) {
		mask := neighborMask(name, wx, wy, m)
		return PixelTileSprites{Base: reg.GetConnectedTileSprite(name, mask)}
	}

	return reg.GetTileSprites(name, tick)
}
