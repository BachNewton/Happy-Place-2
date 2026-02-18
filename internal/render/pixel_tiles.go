package render

import "happy-place-2/internal/maps"

// PixelTileSprite returns the pixel sprites for a tile at world position (wx,wy) at the given tick.
// For connected tiles, it computes the neighbor mask. For others, it uses variant selection via TileHash.
func PixelTileSprite(reg *SpriteRegistry, tile maps.TileDef, wx, wy int, tick uint64, m *maps.Map) PixelTileSprites {
	name := tile.Name

	if !reg.HasTile(name) {
		// Fallback: solid color from ANSI palette
		fgR, fgG, fgB := AnsiToRGB(tile.Fg)
		return PixelTileSprites{Base: FillPixelSprite(fgR, fgG, fgB)}
	}

	if reg.TileIsConnected(name) {
		mask := neighborMask(name, wx, wy, m)
		v := TileHash(wx, wy) % uint(reg.TileVariants(name))
		base := reg.GetConnectedTileSprite(name, mask, v)
		return PixelTileSprites{Base: base}
	}

	v := TileHash(wx, wy) % uint(reg.TileVariants(name))
	return reg.GetTileSprites(name, v, tick)
}
