package render

import (
	"fmt"
	"strings"

	"happy-place-2/internal/maps"
)

const HUDRows = 3

// Cell represents a single terminal cell.
type Cell struct {
	Ch rune
	Fg int
	Bg int
}

// sentinel is a Cell value that will never match a real cell, forcing full draw on first frame.
var sentinel = Cell{Ch: 0, Fg: -1, Bg: -1}

// PlayerInfo is the minimal player data the renderer needs.
type PlayerInfo struct {
	ID    string
	Name  string
	X, Y  int
	Color int
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
) string {
	if termW != e.width || termH != e.height {
		e.Resize(termW, termH)
	}

	// Find the viewer's position
	var viewerX, viewerY int
	var viewerName string
	for _, p := range players {
		if p.ID == viewerID {
			viewerX = p.X
			viewerY = p.Y
			viewerName = p.Name
			break
		}
	}

	vp := NewViewport(viewerX, viewerY, termW, termH, tileMap.Width, tileMap.Height, HUDRows)

	// Clear next buffer
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			e.next[y][x] = Cell{Ch: ' ', Fg: 37, Bg: 0}
		}
	}

	// Fill world tiles into next buffer
	for sy := 0; sy < vp.ViewH && sy < e.height; sy++ {
		for sx := 0; sx < vp.ViewW && sx < e.width; sx++ {
			wx := vp.CamX + sx
			wy := vp.CamY + sy
			tile := tileMap.TileAt(wx, wy)
			e.next[sy][sx] = Cell{Ch: tile.Char, Fg: tile.Fg, Bg: tile.Bg}
		}
	}

	// Overlay players
	for _, p := range players {
		scX, scY := vp.WorldToScreen(p.X, p.Y)
		if scX < 0 || scY < 0 {
			continue
		}
		// Screen coords are 1-based, buffer is 0-based
		bx := scX - 1
		by := scY - 1
		if bx >= 0 && bx < e.width && by >= 0 && by < e.height {
			ch := rune(p.Name[0])
			if p.ID == viewerID {
				ch = '@'
			}
			e.next[by][bx] = Cell{Ch: ch, Fg: p.Color, Bg: 0}
		}
	}

	// Draw HUD in bottom rows
	e.drawHUD(viewerName, len(players), termW, termH)

	// Diff and produce output
	var sb strings.Builder
	sb.Grow(4096)

	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			nc := e.next[y][x]
			if e.firstFrame || nc != e.current[y][x] {
				sb.WriteString(MoveTo(y+1, x+1))
				if nc.Bg > 0 {
					sb.WriteString(fmt.Sprintf("%s%s%c%s", FgColor(nc.Fg), BgColor(nc.Bg), nc.Ch, Reset))
				} else {
					sb.WriteString(fmt.Sprintf("%s%c%s", FgColor(nc.Fg), nc.Ch, Reset))
				}
			}
		}
	}

	// Swap buffers
	e.current, e.next = e.next, e.current
	e.firstFrame = false

	return sb.String()
}

func (e *Engine) drawHUD(playerName string, playerCount, termW, termH int) {
	hudStartY := e.height - HUDRows
	if hudStartY < 0 {
		return
	}

	// Row 1: separator line
	for x := 0; x < e.width; x++ {
		e.next[hudStartY][x] = Cell{Ch: '─', Fg: 90, Bg: 0}
	}

	// Row 2: player info
	info := fmt.Sprintf(" %s | Players online: %d | Map position: visible", playerName, playerCount)
	e.writeHUDLine(hudStartY+1, info, 93)

	// Row 3: controls
	controls := " WASD/Arrows: Move | Q: Quit"
	e.writeHUDLine(hudStartY+2, controls, 37)
}

func (e *Engine) writeHUDLine(row int, text string, fg int) {
	if row < 0 || row >= e.height {
		return
	}
	runes := []rune(text)
	for x := 0; x < e.width; x++ {
		if x < len(runes) {
			e.next[row][x] = Cell{Ch: runes[x], Fg: fg, Bg: 0}
		} else {
			e.next[row][x] = Cell{Ch: ' ', Fg: 37, Bg: 0}
		}
	}
}
