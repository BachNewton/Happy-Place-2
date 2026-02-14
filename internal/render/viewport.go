package render

// Viewport computes camera coordinates for a player's view.
// All coordinates are in world tile units.
type Viewport struct {
	CamX, CamY   int // top-left world coordinate
	ViewW, ViewH int // visible world tiles
}

// NewViewport calculates the camera position centered on the player,
// clamped to map edges. Accounts for TileWidth (each tile = 2 screen cols).
func NewViewport(playerX, playerY, termW, termH, mapW, mapH, hudRows int) Viewport {
	viewW := termW / TileWidth // world tiles visible horizontally
	viewH := termH - hudRows   // world tiles visible vertically (1:1)

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

// IsVisible checks if a world position is within the viewport.
func (v Viewport) IsVisible(wx, wy int) bool {
	return wx >= v.CamX && wx < v.CamX+v.ViewW &&
		wy >= v.CamY && wy < v.CamY+v.ViewH
}

// WorldToLocal converts world coordinates to viewport-local tile coordinates.
// Returns (-1,-1) if not visible.
func (v Viewport) WorldToLocal(wx, wy int) (int, int) {
	if !v.IsVisible(wx, wy) {
		return -1, -1
	}
	return wx - v.CamX, wy - v.CamY
}
