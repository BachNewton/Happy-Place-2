package render

// Viewport computes camera coordinates for a player's view.
type Viewport struct {
	CamX, CamY int // top-left world coordinate
	ViewW, ViewH int // viewport size in cells
}

// NewViewport calculates the camera position centered on the player,
// clamped to map edges. hudRows reserves space for the HUD at the bottom.
func NewViewport(playerX, playerY, termW, termH, mapW, mapH, hudRows int) Viewport {
	viewW := termW
	viewH := termH - hudRows

	camX := playerX - viewW/2
	camY := playerY - viewH/2

	// Clamp to map edges
	if camX < 0 {
		camX = 0
	}
	if camY < 0 {
		camY = 0
	}
	if camX+viewW > mapW {
		camX = mapW - viewW
		if camX < 0 {
			camX = 0
		}
	}
	if camY+viewH > mapH {
		camY = mapH - viewH
		if camY < 0 {
			camY = 0
		}
	}

	return Viewport{
		CamX:  camX,
		CamY:  camY,
		ViewW: viewW,
		ViewH: viewH,
	}
}

// WorldToScreen converts world coordinates to screen coordinates (1-based).
// Returns -1,-1 if the world position is outside the viewport.
func (v Viewport) WorldToScreen(wx, wy int) (int, int) {
	sx := wx - v.CamX + 1
	sy := wy - v.CamY + 1
	if sx < 1 || sx > v.ViewW || sy < 1 || sy > v.ViewH {
		return -1, -1
	}
	return sx, sy
}
