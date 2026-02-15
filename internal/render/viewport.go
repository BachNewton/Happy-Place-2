package render

// Viewport computes camera coordinates for a player's view.
// CamX/CamY are in world tile units. OffsetX/OffsetY are the screen-cell
// position where the first tile's top-left is stamped (0 or negative for
// partial tiles).
type Viewport struct {
	CamX, CamY       int // top-left world tile coordinate
	ViewW, ViewH     int // visible world tiles (including partials)
	OffsetX, OffsetY int // screen offset for the first tile (0 or negative)
}

// NewViewport calculates the camera position centered on the player,
// clamped to map edges. Each tile occupies TileWidth cols x TileHeight rows.
func NewViewport(playerX, playerY, termW, termH, mapW, mapH, hudRows int) Viewport {
	screenW := termW
	screenH := termH - hudRows

	// Center player's tile center on screen center (pixel-level)
	camPixelX := playerX*TileWidth + TileWidth/2 - screenW/2
	camPixelY := playerY*TileHeight + TileHeight/2 - screenH/2

	// Clamp to map pixel edges
	maxPixelX := mapW*TileWidth - screenW
	maxPixelY := mapH*TileHeight - screenH
	if camPixelX < 0 {
		camPixelX = 0
	}
	if camPixelY < 0 {
		camPixelY = 0
	}
	if maxPixelX > 0 && camPixelX > maxPixelX {
		camPixelX = maxPixelX
	}
	if maxPixelY > 0 && camPixelY > maxPixelY {
		camPixelY = maxPixelY
	}

	// Derive tile camera + sub-tile offset
	camX := camPixelX / TileWidth
	camY := camPixelY / TileHeight
	offsetX := -(camPixelX % TileWidth)
	offsetY := -(camPixelY % TileHeight)

	// Tiles needed to cover screen (including partials)
	viewW := (screenW - offsetX + TileWidth - 1) / TileWidth
	viewH := (screenH - offsetY + TileHeight - 1) / TileHeight

	// Don't exceed map bounds
	if camX+viewW > mapW {
		viewW = mapW - camX
	}
	if camY+viewH > mapH {
		viewH = mapH - camY
	}

	return Viewport{
		CamX:    camX,
		CamY:    camY,
		ViewW:   viewW,
		ViewH:   viewH,
		OffsetX: offsetX,
		OffsetY: offsetY,
	}
}

// WorldToScreen converts world tile coordinates to screen-cell coordinates.
func (v Viewport) WorldToScreen(wx, wy int) (int, int) {
	return (wx-v.CamX)*TileWidth + v.OffsetX, (wy-v.CamY)*TileHeight + v.OffsetY
}
