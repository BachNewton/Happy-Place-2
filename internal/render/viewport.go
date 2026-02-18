package render

// PixelViewport computes camera coordinates for pixel-based rendering.
// CamX/CamY are in world tile units. OffsetX/OffsetY are the pixel-buffer
// position where the first tile's top-left is stamped.
type PixelViewport struct {
	CamX, CamY       int // top-left world tile coordinate
	ViewW, ViewH     int // visible world tiles (including partials)
	OffsetX, OffsetY int // pixel offset for the first tile (0 or negative)
}

// NewPixelViewport calculates the camera position centered on the player,
// clamped to map edges. Uses CharTileW cols x CharTileH rows per tile for
// screen-space calculations, but PixelTileW x PixelTileH for pixel-space.
func NewPixelViewport(playerX, playerY, termW, termH, mapW, mapH, hudRows int) PixelViewport {
	screenW := termW
	screenH := termH - hudRows
	// Screen height in pixels (2 pixels per row)
	screenPixH := screenH * 2

	// Center player's tile center on screen center (in char-space for X, pixel-space for Y)
	camCharX := playerX*CharTileW + CharTileW/2 - screenW/2
	camPixelY := playerY*PixelTileH + PixelTileH/2 - screenPixH/2

	// Clamp to map edges
	maxCharX := mapW*CharTileW - screenW
	maxPixelY := mapH*PixelTileH - screenPixH
	if camCharX < 0 {
		camCharX = 0
	}
	if camPixelY < 0 {
		camPixelY = 0
	}
	if maxCharX > 0 && camCharX > maxCharX {
		camCharX = maxCharX
	}
	if maxPixelY > 0 && camPixelY > maxPixelY {
		camPixelY = maxPixelY
	}

	// Derive tile camera + sub-tile offset
	camX := camCharX / CharTileW
	camY := camPixelY / PixelTileH
	offsetX := -(camCharX % CharTileW)
	offsetY := -(camPixelY % PixelTileH)

	// Tiles needed to cover screen (including partials)
	viewW := (screenW - offsetX + CharTileW - 1) / CharTileW
	viewH := (screenPixH - offsetY + PixelTileH - 1) / PixelTileH

	// Don't exceed map bounds
	if camX+viewW > mapW {
		viewW = mapW - camX
	}
	if camY+viewH > mapH {
		viewH = mapH - camY
	}

	return PixelViewport{
		CamX:    camX,
		CamY:    camY,
		ViewW:   viewW,
		ViewH:   viewH,
		OffsetX: offsetX,
		OffsetY: offsetY,
	}
}

// WorldToPixel converts world tile coordinates to pixel-buffer coordinates.
func (v PixelViewport) WorldToPixel(wx, wy int) (int, int) {
	return (wx-v.CamX)*PixelTileW + v.OffsetX, (wy-v.CamY)*PixelTileH + v.OffsetY
}

// WorldToScreen converts world tile coordinates to screen-cell coordinates
// (for HUD/popup positioning that needs char coordinates).
func (v PixelViewport) WorldToScreen(wx, wy int) (int, int) {
	return (wx-v.CamX)*CharTileW + v.OffsetX, (wy-v.CamY)*CharTileH + v.OffsetY/2
}
