package render

import (
	"fmt"
	"strings"

	"happy-place-2/internal/maps"
)

const HUDRows = 3

// Cell represents a single terminal cell with full RGB color.
type Cell struct {
	Ch               rune
	FgR, FgG, FgB   uint8
	BgR, BgG, BgB   uint8
	Bold             bool
}

var sentinel = Cell{Ch: '\x00', FgR: 255, BgB: 255, Bold: true}

// PlayerInfo is the minimal player data the renderer needs.
type PlayerInfo struct {
	ID        string
	Name      string
	X, Y      int
	Color     int // index into PlayerBGColors
	Dir       int // 0=down, 1=up, 2=left, 3=right
	Anim      int // 0=idle, 1=walking
	AnimFrame int // current animation frame
	DebugView bool
}

// Engine is a per-session double-buffer diff renderer.
type Engine struct {
	width, height int
	current       [][]Cell
	next          [][]Cell
	firstFrame    bool
	lastDebugView bool
}

// NewEngine creates a renderer for the given terminal dimensions.
func NewEngine(width, height int) *Engine {
	e := &Engine{
		width:      width,
		height:     height,
		firstFrame: true,
	}
	e.current = e.makeBuffer(sentinel)
	e.next = e.makeBuffer(Cell{})
	return e
}

// Resize adjusts the renderer for a new terminal size.
func (e *Engine) Resize(width, height int) {
	e.width = width
	e.height = height
	e.current = e.makeBuffer(sentinel)
	e.next = e.makeBuffer(Cell{})
	e.firstFrame = true
}

func (e *Engine) makeBuffer(fill Cell) [][]Cell {
	buf := make([][]Cell, e.height)
	for y := 0; y < e.height; y++ {
		buf[y] = make([]Cell, e.width)
		for x := 0; x < e.width; x++ {
			buf[y][x] = fill
		}
	}
	return buf
}

// Render produces the ANSI byte output for the current frame.
func (e *Engine) Render(
	viewerID string,
	tileMap *maps.Map,
	players []PlayerInfo,
	termW, termH int,
	tick uint64,
) string {
	if termW != e.width || termH != e.height {
		e.Resize(termW, termH)
	}

	// Find the viewer
	var viewerX, viewerY int
	var viewerName string
	var viewerColor int
	var viewerDebug bool
	for _, p := range players {
		if p.ID == viewerID {
			viewerX = p.X
			viewerY = p.Y
			viewerName = p.Name
			viewerColor = p.Color
			viewerDebug = p.DebugView
			break
		}
	}

	if viewerDebug != e.lastDebugView {
		e.firstFrame = true
		e.lastDebugView = viewerDebug
	}

	if viewerDebug {
		return e.renderDebugView(viewerColor, tick)
	}

	vp := NewViewport(viewerX, viewerY, termW, termH, tileMap.Width, tileMap.Height, HUDRows)

	// Clear next buffer
	bgCell := Cell{Ch: ' ', BgR: 10, BgG: 10, BgB: 15}
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			e.next[y][x] = bgCell
		}
	}

	// Fill world tiles — each tile is TileWidth x TileHeight screen cells
	for ty := 0; ty < vp.ViewH; ty++ {
		for tx := 0; tx < vp.ViewW; tx++ {
			wx := vp.CamX + tx
			wy := vp.CamY + ty
			tile := tileMap.TileAt(wx, wy)
			sprite := TileSprite(tile, wx, wy, tick)
			e.stampSprite(vp.OffsetX+tx*TileWidth, vp.OffsetY+ty*TileHeight, sprite, false)
		}
	}

	// Overlay players
	for _, p := range players {
		sx, sy := vp.WorldToScreen(p.X, p.Y)
		if sx+TileWidth <= 0 || sx >= termW || sy+TileHeight <= 0 || sy >= (termH-HUDRows) {
			continue
		}
		isSelf := p.ID == viewerID
		sprite := PlayerSprite(p.Dir, p.Anim, p.AnimFrame, p.Color, isSelf, p.Name)
		e.stampSprite(sx, sy, sprite, true)
	}

	// Draw HUD
	e.drawHUD(viewerName, viewerColor, len(players), tileMap.Name)

	// Diff current vs next, emit only changed cells
	var sb strings.Builder
	sb.Grow(16384)

	lastRow, lastCol := -1, -1
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			nc := e.next[y][x]
			if e.firstFrame || nc != e.current[y][x] {
				// Only emit cursor position if not consecutive
				if y != lastRow || x != lastCol {
					sb.WriteString(MoveTo(y+1, x+1))
				}
				WriteCellSGR(&sb, nc)
				lastRow = y
				lastCol = x + 1
			}
		}
	}

	if sb.Len() > 0 {
		sb.WriteString(Reset)
	}

	// Swap buffers
	e.current, e.next = e.next, e.current
	e.firstFrame = false

	return sb.String()
}

// stampSprite writes a sprite into the buffer at screen position (sx, sy).
// When transparent is true, SpriteCell.Transparent cells are skipped.
func (e *Engine) stampSprite(sx, sy int, sprite Sprite, transparent bool) {
	for row := 0; row < TileHeight; row++ {
		screenY := sy + row
		if screenY < 0 || screenY >= e.height {
			continue
		}
		for col := 0; col < TileWidth; col++ {
			screenX := sx + col
			if screenX < 0 || screenX >= e.width {
				continue
			}
			sc := sprite[row][col]
			if transparent && sc.Transparent {
				continue
			}
			e.next[screenY][screenX] = sc.Cell
		}
	}
}

// --- HUD ---

func (e *Engine) drawHUD(playerName string, playerColor, playerCount int, mapName string) {
	hudY := e.height - HUDRows
	if hudY < 0 {
		return
	}

	// Row 1: separator — thin gradient line
	for x := 0; x < e.width; x++ {
		// Gradient from teal to dark
		t := uint8(60 - x*40/max(e.width, 1))
		e.next[hudY][x] = Cell{
			Ch: '━', FgR: 40 + t, FgG: 70 + t, FgB: 90 + t,
			BgR: 15, BgG: 18, BgB: 28,
		}
	}

	// Row 2: info bar
	barBgR, barBgG, barBgB := uint8(18), uint8(22), uint8(38)

	// Fill bar background
	for x := 0; x < e.width; x++ {
		e.next[hudY+1][x] = Cell{Ch: ' ', BgR: barBgR, BgG: barBgG, BgB: barBgB}
	}

	// Build info line pieces
	colorIdx := playerColor % len(PlayerBGColors)
	pR, pG, pB := PlayerBGColors[colorIdx][0], PlayerBGColors[colorIdx][1], PlayerBGColors[colorIdx][2]
	// Brighten for text use
	pR = pR + (255-pR)/3
	pG = pG + (255-pG)/3
	pB = pB + (255-pB)/3

	infoLine := fmt.Sprintf(" %s  \u2502  %s  \u2502  %d Online", playerName, mapName, playerCount)
	e.writeHUDStyledLine(hudY+1, infoLine, barBgR, barBgG, barBgB, playerName, pR, pG, pB)

	// Row 3: controls bar
	ctrlBgR, ctrlBgG, ctrlBgB := uint8(15), uint8(18), uint8(30)
	for x := 0; x < e.width; x++ {
		e.next[hudY+2][x] = Cell{Ch: ' ', BgR: ctrlBgR, BgG: ctrlBgG, BgB: ctrlBgB}
	}
	controls := " \u2190\u2191\u2193\u2192/WASD Move  \u2502  Q Quit"
	e.writeHUDTextLine(hudY+2, controls, 130, 130, 145, ctrlBgR, ctrlBgG, ctrlBgB)
}

func (e *Engine) writeHUDStyledLine(row int, text string, bgR, bgG, bgB uint8, highlight string, hR, hG, hB uint8) {
	if row < 0 || row >= e.height {
		return
	}
	runes := []rune(text)
	highlightRunes := []rune(highlight)
	highlightStart := -1

	// Find highlight position
	for i := range runes {
		match := true
		for j, hr := range highlightRunes {
			if i+j >= len(runes) || runes[i+j] != hr {
				match = false
				break
			}
		}
		if match {
			highlightStart = i
			break
		}
	}

	for x := 0; x < e.width; x++ {
		if x < len(runes) {
			fgR, fgG, fgB := uint8(180), uint8(180), uint8(195)
			bold := false
			// Highlight player name
			if highlightStart >= 0 && x >= highlightStart && x < highlightStart+len(highlightRunes) {
				fgR, fgG, fgB = hR, hG, hB
				bold = true
			}
			e.next[row][x] = Cell{Ch: runes[x], FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: bold}
		} else {
			e.next[row][x] = Cell{Ch: ' ', BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}
}

func (e *Engine) writeHUDTextLine(row int, text string, fgR, fgG, fgB, bgR, bgG, bgB uint8) {
	if row < 0 || row >= e.height {
		return
	}
	runes := []rune(text)
	for x := 0; x < e.width; x++ {
		if x < len(runes) {
			e.next[row][x] = Cell{Ch: runes[x], FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
		} else {
			e.next[row][x] = Cell{Ch: ' ', BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}
}

// renderDebugView draws a grid of all tile sprites and player direction sprites.
func (e *Engine) renderDebugView(viewerColor int, tick uint64) string {
	// Clear buffer with dark background
	bgCell := Cell{Ch: ' ', BgR: 18, BgG: 18, BgB: 24}
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			e.next[y][x] = bgCell
		}
	}

	// Title row
	title := "SPRITE DEBUG (~ to close)"
	titleRunes := []rune(title)
	for i, r := range titleRunes {
		if i < e.width {
			e.next[0][i+1] = Cell{Ch: r, FgR: 255, FgG: 220, FgB: 100, BgR: 18, BgG: 18, BgB: 24, Bold: true}
		}
	}

	// Layout constants
	gap := 1
	rowHeight := TileHeight + 2 // label row + sprite + gap row

	// Helper to write a label at screen position
	writeLabel := func(row, col int, label string) {
		for i, r := range []rune(label) {
			x := col + i
			if x >= 0 && x < e.width && row >= 0 && row < e.height {
				e.next[row][x] = Cell{Ch: r, FgR: 160, FgG: 160, FgB: 175, BgR: 18, BgG: 18, BgB: 24}
			}
		}
	}

	// Flow layout cursor
	curX := 0
	curY := 2

	// placeGroup places a labeled sprite group, advancing the cursor.
	// Returns the x position where sprites start.
	placeGroup := func(label string, width int) (int, int) {
		// Wrap if this group won't fit on the current row
		if curX > 0 && curX+width > e.width {
			curX = 0
			curY += rowHeight
		}
		sx, sy := curX, curY
		writeLabel(sy, sx, label)
		curX += width + gap*2
		return sx, sy
	}

	// --- Tile sprites with variants ---
	for i := range tileList {
		entry := &tileList[i]
		groupWidth := entry.variants*TileWidth + (entry.variants-1)*gap

		sx, sy := placeGroup(entry.name, groupWidth)
		for v := 0; v < entry.variants; v++ {
			wx, wy := variantCoord(v, entry.variants)
			sprite := entry.fn(wx, wy, tick)
			e.stampSprite(sx+v*(TileWidth+gap), sy+1, sprite, false)
		}
	}

	// --- Player sprites ---
	// Start on a new row after tiles
	curX = 0
	curY += rowHeight

	dirNames := []string{"down", "up", "left", "right"}
	for i, dName := range dirNames {
		sx, sy := placeGroup(dName, TileWidth)
		sprite := PlayerSprite(i, 0, 0, viewerColor, true, "Debug")
		e.stampSprite(sx, sy+1, sprite, true)
	}

	// Diff and emit
	var sb strings.Builder
	sb.Grow(16384)

	lastRow, lastCol := -1, -1
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			nc := e.next[y][x]
			if e.firstFrame || nc != e.current[y][x] {
				if y != lastRow || x != lastCol {
					sb.WriteString(MoveTo(y+1, x+1))
				}
				WriteCellSGR(&sb, nc)
				lastRow = y
				lastCol = x + 1
			}
		}
	}

	if sb.Len() > 0 {
		sb.WriteString(Reset)
	}

	e.current, e.next = e.next, e.current
	e.firstFrame = false

	return sb.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
