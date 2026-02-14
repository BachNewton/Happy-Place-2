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
	ID    string
	Name  string
	X, Y  int
	Color int // index into PlayerBGColors
}

// Engine is a per-session double-buffer diff renderer.
type Engine struct {
	width, height int
	current       [][]Cell
	next          [][]Cell
	firstFrame    bool
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
	for _, p := range players {
		if p.ID == viewerID {
			viewerX = p.X
			viewerY = p.Y
			viewerName = p.Name
			viewerColor = p.Color
			break
		}
	}

	vp := NewViewport(viewerX, viewerY, termW, termH, tileMap.Width, tileMap.Height, HUDRows)

	// Clear next buffer
	bgCell := Cell{Ch: ' ', BgR: 10, BgG: 10, BgB: 15}
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			e.next[y][x] = bgCell
		}
	}

	// Fill world tiles (each tile = TileWidth screen columns)
	for ty := 0; ty < vp.ViewH && ty < e.height; ty++ {
		for tx := 0; tx < vp.ViewW; tx++ {
			wx := vp.CamX + tx
			wy := vp.CamY + ty
			screenCol := tx * TileWidth
			if screenCol+1 >= e.width {
				break
			}

			tile := tileMap.TileAt(wx, wy)
			left, right := themeTile(tile, wx, wy, tick)
			e.next[ty][screenCol] = left
			e.next[ty][screenCol+1] = right
		}
	}

	// Overlay players (each player occupies TileWidth cells)
	for _, p := range players {
		lx, ly := vp.WorldToLocal(p.X, p.Y)
		if lx < 0 {
			continue
		}
		screenCol := lx * TileWidth
		if screenCol+1 >= e.width || ly >= e.height {
			continue
		}

		ch := rune(p.Name[0])
		if p.ID == viewerID {
			ch = '@'
		}

		colorIdx := p.Color % len(PlayerBGColors)
		bgR, bgG, bgB := PlayerBGColors[colorIdx][0], PlayerBGColors[colorIdx][1], PlayerBGColors[colorIdx][2]
		fgR, fgG, fgB := PlayerFGColors[colorIdx][0], PlayerFGColors[colorIdx][1], PlayerFGColors[colorIdx][2]

		e.next[ly][screenCol] = Cell{
			Ch: ch, FgR: fgR, FgG: fgG, FgB: fgB,
			BgR: bgR, BgG: bgG, BgB: bgB, Bold: true,
		}
		e.next[ly][screenCol+1] = Cell{
			Ch: ' ', FgR: fgR, FgG: fgG, FgB: fgB,
			BgR: bgR, BgG: bgG, BgB: bgB,
		}
	}

	// Draw HUD
	e.drawHUD(viewerName, viewerColor, len(players), tileMap.Name)

	// Diff current vs next, emit only changed cells
	var sb strings.Builder
	sb.Grow(8192)

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

// --- Tile Theming ---

func themeTile(tile maps.TileDef, wx, wy int, tick uint64) (Cell, Cell) {
	v := uint(wx*7+wy*13) % 4

	switch tile.Name {
	case "grass":
		return themeGrass(v)
	case "wall":
		return themeWall(v)
	case "water":
		return themeWater(wx, wy, tick)
	case "tree":
		return themeTree(v)
	case "path":
		return themePath(v)
	case "door":
		return themeDoor()
	case "floor":
		return themeFloor(v)
	case "fence":
		return themeFence(v)
	default:
		return themeFallback(tile)
	}
}

func themeGrass(v uint) (Cell, Cell) {
	bgR, bgG, bgB := uint8(28), uint8(65), uint8(28)
	type grassVar struct {
		l, r       rune
		fr, fg, fb uint8
	}
	vars := [4]grassVar{
		{',', ' ', 60, 135, 50},
		{' ', '.', 50, 115, 42},
		{'.', ' ', 70, 145, 55},
		{' ', ' ', 55, 125, 48},
	}
	g := vars[v]
	return Cell{Ch: g.l, FgR: g.fr, FgG: g.fg, FgB: g.fb, BgR: bgR, BgG: bgG, BgB: bgB},
		Cell{Ch: g.r, FgR: g.fr, FgG: g.fg, FgB: g.fb, BgR: bgR, BgG: bgG, BgB: bgB}
}

func themeWall(v uint) (Cell, Cell) {
	type wallVar struct {
		l, r       rune
		fr, fg, fb uint8
		br, bg, bb uint8
	}
	vars := [4]wallVar{
		{'█', '▓', 115, 115, 125, 60, 60, 70},
		{'▓', '█', 110, 110, 120, 58, 58, 68},
		{'█', '█', 120, 120, 130, 62, 62, 72},
		{'▓', '▓', 105, 105, 115, 56, 56, 66},
	}
	w := vars[v]
	return Cell{Ch: w.l, FgR: w.fr, FgG: w.fg, FgB: w.fb, BgR: w.br, BgG: w.bg, BgB: w.bb},
		Cell{Ch: w.r, FgR: w.fr, FgG: w.fg, FgB: w.fb, BgR: w.br, BgG: w.bg, BgB: w.bb}
}

func themeWater(wx, wy int, tick uint64) (Cell, Cell) {
	bgR, bgG, bgB := uint8(15), uint8(38), uint8(95)
	fgR, fgG, fgB := uint8(70), uint8(130), uint8(210)

	// Animate: shift pattern based on tick and position
	frame := uint((int(tick/8) + wx*3 + wy*5)) % 4
	type waterFrame struct{ l, r rune }
	frames := [4]waterFrame{
		{'~', ' '},
		{' ', '~'},
		{'≈', ' '},
		{' ', '≈'},
	}
	f := frames[frame]

	// Subtle color shimmer
	shimmer := uint8((int(tick/6) + wx + wy*2) % 3)
	fgB += shimmer * 15

	return Cell{Ch: f.l, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB},
		Cell{Ch: f.r, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
}

func themeTree(v uint) (Cell, Cell) {
	bgR, bgG, bgB := uint8(22), uint8(55), uint8(22)
	type treeVar struct {
		ch         rune
		fr, fg, fb uint8
	}
	vars := [4]treeVar{
		{'♣', 35, 170, 35},
		{'♠', 30, 155, 30},
		{'♣', 40, 180, 40},
		{'♠', 32, 160, 32},
	}
	t := vars[v]
	return Cell{Ch: t.ch, FgR: t.fr, FgG: t.fg, FgB: t.fb, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true},
		Cell{Ch: ' ', FgR: t.fr, FgG: t.fg, FgB: t.fb, BgR: bgR, BgG: bgG, BgB: bgB}
}

func themePath(v uint) (Cell, Cell) {
	bgR, bgG, bgB := uint8(120), uint8(95), uint8(55)
	fgR, fgG, fgB := uint8(150), uint8(120), uint8(75)
	type pathVar struct{ l, r rune }
	vars := [4]pathVar{
		{'·', ' '},
		{' ', '·'},
		{' ', ' '},
		{'·', '·'},
	}
	p := vars[v]
	return Cell{Ch: p.l, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB},
		Cell{Ch: p.r, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
}

func themeDoor() (Cell, Cell) {
	bgR, bgG, bgB := uint8(110), uint8(75), uint8(30)
	fgR, fgG, fgB := uint8(210), uint8(170), uint8(60)
	return Cell{Ch: '▐', FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true},
		Cell{Ch: '▌', FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true}
}

func themeFloor(v uint) (Cell, Cell) {
	bgR, bgG, bgB := uint8(72), uint8(52), uint8(32)
	fgR, fgG, fgB := uint8(92), uint8(68), uint8(42)
	type floorVar struct{ l, r rune }
	vars := [4]floorVar{
		{' ', ' '},
		{'·', ' '},
		{' ', ' '},
		{' ', '·'},
	}
	f := vars[v]
	return Cell{Ch: f.l, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB},
		Cell{Ch: f.r, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
}

func themeFence(v uint) (Cell, Cell) {
	bgR, bgG, bgB := uint8(28), uint8(65), uint8(28) // grass behind
	fgR, fgG, fgB := uint8(155), uint8(115), uint8(55)
	ch := rune('│')
	if v%2 == 1 {
		ch = '┃'
	}
	return Cell{Ch: ch, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true},
		Cell{Ch: ' ', FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
}

func themeFallback(tile maps.TileDef) (Cell, Cell) {
	fgR, fgG, fgB := AnsiToRGB(tile.Fg)
	bgR, bgG, bgB := uint8(10), uint8(10), uint8(15)
	if tile.Bg > 0 {
		bgR, bgG, bgB = AnsiToRGB(tile.Bg)
	}
	return Cell{Ch: tile.Char, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB},
		Cell{Ch: ' ', FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
