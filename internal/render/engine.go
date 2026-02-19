package render

import (
	"fmt"

	"happy-place-2/internal/maps"
)

const HUDRows = 4

// Cell represents a single terminal cell with full RGB color.
type Cell struct {
	Ch               rune
	FgR, FgG, FgB   uint8
	BgR, BgG, BgB   uint8
	Bold             bool
}

var sentinel = Cell{Ch: '\x00', FgR: 255, BgB: 255, Bold: true}

// InteractionPopup is the data needed to render an interaction popup.
type InteractionPopup struct {
	WorldX, WorldY int
	Text           string
}

// PlayerInfo is the minimal player data the renderer needs.
type PlayerInfo struct {
	ID                string
	Name              string
	X, Y              int
	Color             int // index into PlayerBGColors
	Dir               int // 0=down, 1=up, 2=left, 3=right
	Anim              int // 0=idle, 1=walking
	AnimFrame         int // current animation frame
	DebugView         bool
	DebugPage         int
	DebugTileOverlay  bool
	ActiveInteraction *InteractionPopup

	HP, MaxHP           int
	Stamina, MaxStamina int
	MP, MaxMP           int
	EXP                 int
	Level               int
	InCombat            bool
	CombatTransition    int
}

// CombatRenderData holds combat state for the renderer.
type CombatRenderData struct {
	Phase         int // maps to game.CombatPhase
	Round         int
	Enemies       []CombatEnemy
	Players       []CombatPlayer
	CurrentTurn   string // player ID whose turn it is
	TurnTimer     int    // ticks remaining
	Log           []string
	ViewerID      string
	Transitioning bool
	ViewerAction  int   // selected action (1-3, 0=none)
	ViewerTarget  int   // selected enemy target index
}

// CombatEnemy is enemy data for rendering.
type CombatEnemy struct {
	Label string
	HP    int
	MaxHP int
	ID    int
	Alive bool
}

// CombatPlayer is player data for combat rendering.
type CombatPlayer struct {
	ID       string
	Name     string
	HP       int
	MaxHP    int
	Alive    bool
	Color    int
	IsViewer bool
}

// Engine is a per-session double-buffer diff renderer.
type Engine struct {
	width, height int
	current       [][]Cell
	next          [][]Cell
	firstFrame    bool
	lastDebugView bool
	lastDebugPage int
	lastInCombat  bool

	// Pixel buffer for half-block rendering (world area only)
	pixelBuf [][]Pixel
	pixBufW  int // pixel columns (= terminal columns)
	pixBufH  int // pixel rows (= (termH - HUDRows) * 2)
	sprites  *SpriteRegistry
}

// NewEngine creates a renderer for the given terminal dimensions.
func NewEngine(width, height int, sprites *SpriteRegistry) *Engine {
	e := &Engine{
		width:      width,
		height:     height,
		firstFrame: true,
		sprites:    sprites,
	}
	e.current = e.makeBuffer(sentinel)
	e.next = e.makeBuffer(Cell{})
	e.initPixelBuf()
	return e
}

// Resize adjusts the renderer for a new terminal size.
func (e *Engine) Resize(width, height int) {
	e.width = width
	e.height = height
	e.current = e.makeBuffer(sentinel)
	e.next = e.makeBuffer(Cell{})
	e.initPixelBuf()
	e.firstFrame = true
}

// initPixelBuf allocates the pixel buffer for the world area.
func (e *Engine) initPixelBuf() {
	e.pixBufW = e.width
	worldRows := e.height - HUDRows
	if worldRows < 0 {
		worldRows = 0
	}
	e.pixBufH = worldRows * 2 // 2 pixels per terminal row
	e.pixelBuf = make([][]Pixel, e.pixBufH)
	for y := 0; y < e.pixBufH; y++ {
		e.pixelBuf[y] = make([]Pixel, e.pixBufW)
	}
}

// clearPixelBuf fills the pixel buffer with a solid color.
func (e *Engine) clearPixelBuf(r, g, b uint8) {
	p := P(r, g, b)
	for y := 0; y < e.pixBufH; y++ {
		for x := 0; x < e.pixBufW; x++ {
			e.pixelBuf[y][x] = p
		}
	}
}

// stampPixelSprite writes a pixel sprite into the pixel buffer at position (px, py).
// When transparent is true, transparent pixels are skipped.
func (e *Engine) stampPixelSprite(px, py int, sprite PixelSprite, transparent bool) {
	for row := 0; row < PixelTileH; row++ {
		bufY := py + row
		if bufY < 0 || bufY >= e.pixBufH {
			continue
		}
		for col := 0; col < PixelTileW; col++ {
			bufX := px + col
			if bufX < 0 || bufX >= e.pixBufW {
				continue
			}
			p := sprite[row][col]
			if transparent && p.Transparent {
				continue
			}
			e.pixelBuf[bufY][bufX] = p
		}
	}
}

// collapsePixelBuf converts pixel pairs into half-block Cell values in next[][].
// Each terminal row covers 2 pixel rows. The top pixel becomes the background color
// and the bottom pixel becomes the foreground color of '▄' (U+2584).
func (e *Engine) collapsePixelBuf() {
	worldRows := e.height - HUDRows
	if worldRows < 0 {
		worldRows = 0
	}
	for row := 0; row < worldRows; row++ {
		topPixRow := row * 2
		botPixRow := row*2 + 1
		for col := 0; col < e.width; col++ {
			if col >= e.pixBufW {
				break
			}
			var top, bot Pixel
			if topPixRow < e.pixBufH {
				top = e.pixelBuf[topPixRow][col]
			}
			if botPixRow < e.pixBufH {
				bot = e.pixelBuf[botPixRow][col]
			}
			e.next[row][col] = Cell{
				Ch:  '▄',
				FgR: bot.R, FgG: bot.G, FgB: bot.B,
				BgR: top.R, BgG: top.G, BgB: top.B,
			}
		}
	}
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
	totalPlayers int,
	combat *CombatRenderData,
) string {
	if termW != e.width || termH != e.height {
		e.Resize(termW, termH)
	}

	// Find the viewer
	var viewerX, viewerY int
	var viewerName string
	var viewerColor int
	var viewerDebug bool
	var viewerDebugPage int
	var viewerTileOverlay bool
	var viewerHP, viewerMaxHP int
	var viewerSTA, viewerMaxSTA int
	var viewerMP, viewerMaxMP int
	var viewerEXP, viewerLevel int
	for _, p := range players {
		if p.ID == viewerID {
			viewerX = p.X
			viewerY = p.Y
			viewerName = p.Name
			viewerColor = p.Color
			viewerDebug = p.DebugView
			viewerDebugPage = p.DebugPage
			viewerTileOverlay = p.DebugTileOverlay
			viewerHP = p.HP
			viewerMaxHP = p.MaxHP
			viewerSTA = p.Stamina
			viewerMaxSTA = p.MaxStamina
			viewerMP = p.MP
			viewerMaxMP = p.MaxMP
			viewerEXP = p.EXP
			viewerLevel = p.Level
			break
		}
	}

	if viewerDebug != e.lastDebugView {
		e.firstFrame = true
		e.lastDebugView = viewerDebug
	}
	if viewerDebugPage != e.lastDebugPage {
		e.firstFrame = true
		e.lastDebugPage = viewerDebugPage
	}

	inCombat := combat != nil
	if inCombat != e.lastInCombat {
		e.firstFrame = true
		e.lastInCombat = inCombat
	}

	statsInfo := HUDStats{
		HP: viewerHP, MaxHP: viewerMaxHP,
		Stamina: viewerSTA, MaxStamina: viewerMaxSTA,
		MP: viewerMP, MaxMP: viewerMaxMP,
		EXP: viewerEXP, Level: viewerLevel,
	}

	if viewerDebug {
		return e.renderDebugView(viewerColor, viewerDebugPage, tick)
	}

	if combat != nil {
		return e.renderCombatView(combat, viewerName, viewerColor, totalPlayers, tick, statsInfo)
	}

	vp := NewPixelViewport(viewerX, viewerY, termW, termH, tileMap.Width, tileMap.Height, HUDRows)

	// Clear pixel buffer with background color
	e.clearPixelBuf(10, 10, 15)

	// --- Pass 1: Ground tiles + collect overlays ---
	const maxOverlayDY = 3 // scan extra rows below viewport for tiles whose overlays reach in

	type pendingPixelOverlay struct {
		px, py int
		sprite PixelSprite
	}
	var overlays []pendingPixelOverlay

	scanH := vp.ViewH + maxOverlayDY
	for ty := 0; ty < scanH; ty++ {
		for tx := 0; tx < vp.ViewW; tx++ {
			wx := vp.CamX + tx
			wy := vp.CamY + ty
			if wx < 0 || wx >= tileMap.Width || wy < 0 || wy >= tileMap.Height {
				continue
			}
			tile := tileMap.TileAt(wx, wy)
			ts := PixelTileSprite(e.sprites, tile, wx, wy, tick, tileMap)
			px := vp.OffsetX + tx*PixelTileW
			py := vp.OffsetY + ty*PixelTileH

			// Only stamp base for tiles within the visible viewport
			if ty < vp.ViewH {
				e.stampPixelSprite(px, py, ts.Base, false)
			}

			// Collect overlays — rendered after players
			for _, ov := range ts.Overlays {
				ovPY := py - ov.DY*PixelTileH
				overlays = append(overlays, pendingPixelOverlay{px: px, py: ovPY, sprite: ov.Sprite})
			}
		}
	}

	// --- Pass 2: Players ---
	var viewerPopup *InteractionPopup
	for _, p := range players {
		px, py := vp.WorldToPixel(p.X, p.Y)
		if px+PixelTileW <= 0 || px >= e.pixBufW || py+PixelTileH <= 0 || py >= e.pixBufH {
			continue
		}
		isSelf := p.ID == viewerID
		if isSelf && p.ActiveInteraction != nil {
			viewerPopup = p.ActiveInteraction
		}
		sprite := e.sprites.GetPlayerSprite(p.Dir, p.Color)
		e.stampPixelSprite(px, py, sprite, true)
	}

	// --- Pass 3: Overlays (on top of players) ---
	for _, ov := range overlays {
		e.stampPixelSprite(ov.px, ov.py, ov.sprite, true)
	}

	// Collapse pixel buffer into half-block cells
	e.collapsePixelBuf()

	// Tile debug overlay: type letter + (X,Y) on each tile
	if viewerTileOverlay {
		e.drawTileOverlay(vp, tileMap)
	}

	// Draw interaction popup above sign tile (character-based, on top of collapsed cells)
	if viewerPopup != nil {
		e.drawInteractionPopupPixel(viewerPopup, vp, termH)
	}

	// Draw HUD (character-based, writes directly to next)
	e.drawHUD(viewerName, viewerColor, totalPlayers, tileMap.Name, statsInfo)

	return e.emitDiff()
}

// --- Tile Debug Overlay ---

// drawTileOverlay renders tile type letter + (X,Y) world coordinates on each visible tile.
// Toggled with 'T' key. Draws character-based text on top of collapsed half-block cells.
func (e *Engine) drawTileOverlay(vp PixelViewport, tileMap *maps.Map) {
	worldRows := e.height - HUDRows
	if worldRows < 0 {
		worldRows = 0
	}

	// Semi-transparent label colors
	bgR, bgG, bgB := uint8(0), uint8(0), uint8(0)
	fgR, fgG, fgB := uint8(255), uint8(255), uint8(255)

	setOverlayCell := func(sx, sy int, ch rune) {
		if sx >= 0 && sx < e.width && sy >= 0 && sy < worldRows {
			e.next[sy][sx] = Cell{Ch: ch, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}

	for ty := 0; ty < vp.ViewH; ty++ {
		for tx := 0; tx < vp.ViewW; tx++ {
			wx := vp.CamX + tx
			wy := vp.CamY + ty

			if wx < 0 || wx >= tileMap.Width || wy < 0 || wy >= tileMap.Height {
				continue
			}

			tile := tileMap.TileAt(wx, wy)

			// Screen position of this tile (in char coords)
			screenX := (vp.OffsetX + tx*PixelTileW)    // pixel X = char X (1:1)
			screenY := (vp.OffsetY + ty*PixelTileH) / 2 // pixel Y → char row (2 pixels per row)

			// Tile type letter — first char of name, uppercased
			letter := '?'
			if len(tile.Name) > 0 {
				r := rune(tile.Name[0])
				if r >= 'a' && r <= 'z' {
					r -= 32
				}
				letter = r
			}

			// Center the letter in the tile (CharTileW=16 wide, CharTileH=8 tall)
			centerX := screenX + CharTileW/2
			centerY := screenY + CharTileH/2 - 1
			setOverlayCell(centerX, centerY, letter)

			// (X,Y) coordinates below the letter
			coordStr := fmt.Sprintf("%d,%d", wx, wy)
			coordX := screenX + (CharTileW-len(coordStr))/2
			coordY := centerY + 1
			for i, r := range coordStr {
				setOverlayCell(coordX+i, coordY, r)
			}
		}
	}
}

// --- Interaction Popup ---

// drawInteractionPopupPixel draws the popup using PixelViewport tile dimensions.
func (e *Engine) drawInteractionPopupPixel(popup *InteractionPopup, vp PixelViewport, termH int) {
	signSX, signSY := vp.WorldToScreen(popup.WorldX, popup.WorldY)

	textRunes := []rune(popup.Text)
	popupW := len(textRunes) + 4 // "│ " + text + " │"
	popupH := 3                  // top border, text, bottom border

	// Horizontal: center on sign tile, clamp to screen
	popupX := signSX + (CharTileW-popupW)/2
	if popupX < 0 {
		popupX = 0
	}
	if popupX+popupW > e.width {
		popupX = e.width - popupW
	}

	hudTop := termH - HUDRows

	// Vertical: prefer above the sign tile
	popupY := signSY - popupH
	if popupY < 0 {
		// Not enough room above — try below
		popupY = signSY + CharTileH
	}
	// If popup overlaps HUD, try above instead; if still no room, skip
	if popupY+popupH > hudTop {
		popupY = signSY - popupH
		if popupY < 0 {
			return
		}
	}

	// Colors: warm border, dark bg, light text
	borderR, borderG, borderB := uint8(200), uint8(180), uint8(120)
	bgR, bgG, bgB := uint8(30), uint8(25), uint8(45)
	textR, textG, textB := uint8(240), uint8(230), uint8(200)

	setCell := func(sx, sy int, ch rune, fgR, fgG, fgB, bgR, bgG, bgB uint8) {
		if sx >= 0 && sx < e.width && sy >= 0 && sy < e.height {
			e.next[sy][sx] = Cell{Ch: ch, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}

	// Top border: ┌──...──┐
	setCell(popupX, popupY, '┌', borderR, borderG, borderB, bgR, bgG, bgB)
	for i := 1; i < popupW-1; i++ {
		setCell(popupX+i, popupY, '─', borderR, borderG, borderB, bgR, bgG, bgB)
	}
	setCell(popupX+popupW-1, popupY, '┐', borderR, borderG, borderB, bgR, bgG, bgB)

	// Middle row: │ text │
	midY := popupY + 1
	setCell(popupX, midY, '│', borderR, borderG, borderB, bgR, bgG, bgB)
	setCell(popupX+1, midY, ' ', textR, textG, textB, bgR, bgG, bgB)
	for i, r := range textRunes {
		setCell(popupX+2+i, midY, r, textR, textG, textB, bgR, bgG, bgB)
	}
	setCell(popupX+popupW-2, midY, ' ', textR, textG, textB, bgR, bgG, bgB)
	setCell(popupX+popupW-1, midY, '│', borderR, borderG, borderB, bgR, bgG, bgB)

	// Bottom border: └──...──┘
	botY := popupY + 2
	setCell(popupX, botY, '└', borderR, borderG, borderB, bgR, bgG, bgB)
	for i := 1; i < popupW-1; i++ {
		setCell(popupX+i, botY, '─', borderR, borderG, borderB, bgR, bgG, bgB)
	}
	setCell(popupX+popupW-1, botY, '┘', borderR, borderG, borderB, bgR, bgG, bgB)
}

// HUDStats holds player stats for HUD display.
type HUDStats struct {
	HP, MaxHP           int
	Stamina, MaxStamina int
	MP, MaxMP           int
	EXP                 int
	Level               int
}

// --- HUD ---

func (e *Engine) drawHUD(playerName string, playerColor, playerCount int, mapName string, stats HUDStats) {
	hudY := e.height - HUDRows
	if hudY < 0 {
		return
	}

	splitCol := e.width / 2
	bgR, bgG, bgB := uint8(15), uint8(18), uint8(30)

	// Row 0: separator — thin gradient line
	for x := 0; x < e.width; x++ {
		t := uint8(60 - x*40/max(e.width, 1))
		e.next[hudY][x] = Cell{
			Ch: '━', FgR: 40 + t, FgG: 70 + t, FgB: 90 + t,
			BgR: bgR, BgG: bgG, BgB: bgB,
		}
	}

	// Fill rows 1-3 with background and vertical separator
	for row := 1; row <= 3; row++ {
		y := hudY + row
		if y >= e.height {
			break
		}
		for x := 0; x < e.width; x++ {
			e.next[y][x] = Cell{Ch: ' ', BgR: bgR, BgG: bgG, BgB: bgB}
		}
		if splitCol > 0 && splitCol < e.width {
			e.next[y][splitCol] = Cell{Ch: '│', FgR: 50, FgG: 60, FgB: 80, BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}

	// --- Left column ---
	colorIdx := playerColor % len(PlayerBGColors)
	pR, pG, pB := PlayerBGColors[colorIdx][0], PlayerBGColors[colorIdx][1], PlayerBGColors[colorIdx][2]
	pR = pR + (255-pR)/3
	pG = pG + (255-pG)/3
	pB = pB + (255-pB)/3

	// Row 1: world info — player name, map, online count
	row1 := hudY + 1
	col := e.writeText(row1, 1, splitCol, playerName, pR, pG, pB, bgR, bgG, bgB, true)
	col = e.writeText(row1, col, splitCol, "  │  ", 60, 65, 85, bgR, bgG, bgB, false)
	col = e.writeText(row1, col, splitCol, mapName, 180, 180, 195, bgR, bgG, bgB, false)
	col = e.writeText(row1, col, splitCol, "  │  ", 60, 65, 85, bgR, bgG, bgB, false)
	e.writeText(row1, col, splitCol, fmt.Sprintf("%d Online", playerCount), 180, 180, 195, bgR, bgG, bgB, false)

	// Row 2: EXP / Level
	row2 := hudY + 2
	lvText := fmt.Sprintf("Lv %d", stats.Level)
	col = e.writeText(row2, 1, splitCol, lvText, 100, 220, 220, bgR, bgG, bgB, true)
	col += 2
	expInLevel := stats.EXP % 50
	expNums := fmt.Sprintf("%d/%d", expInLevel, 50)
	expBarWidth := splitCol - col - len("EXP") - 1 - 1 - len(expNums)
	if expBarWidth < 4 {
		expBarWidth = 4
	}
	e.drawStatBar(row2, col, "EXP", expInLevel, 50, expBarWidth,
		60, 200, 180, 50, 190, 160, bgR, bgG, bgB)

	// Row 3: controls
	row3 := hudY + 3
	e.writeText(row3, 1, splitCol, "←↑↓→/WASD Move  │  T Tiles  │  Q Quit", 130, 130, 145, bgR, bgG, bgB, false)

	// --- Right column: stat bars ---
	rightStart := splitCol + 2
	hpNums := fmt.Sprintf("%d/%d", stats.HP, stats.MaxHP)
	staNums := fmt.Sprintf("%d/%d", stats.Stamina, stats.MaxStamina)
	mpNums := fmt.Sprintf("%d/%d", stats.MP, stats.MaxMP)
	maxNumLen := max(len(hpNums), max(len(staNums), len(mpNums)))
	barWidth := (e.width - rightStart) - 9 - maxNumLen
	if barWidth < 4 {
		barWidth = 4
	}

	hpFillR, hpFillG, hpFillB := hpBarColor(stats.HP, stats.MaxHP)
	e.drawStatBar(row1, rightStart, "Health ", stats.HP, stats.MaxHP, barWidth,
		255, 80, 80, hpFillR, hpFillG, hpFillB, bgR, bgG, bgB)
	e.drawStatBar(row2, rightStart, "Stamina", stats.Stamina, stats.MaxStamina, barWidth,
		240, 190, 60, 210, 170, 50, bgR, bgG, bgB)
	e.drawStatBar(row3, rightStart, "Magic  ", stats.MP, stats.MaxMP, barWidth,
		100, 140, 255, 90, 110, 240, bgR, bgG, bgB)
}

// hpBarColor returns the fill color for an HP bar based on current/max ratio.
func hpBarColor(current, maxHP int) (uint8, uint8, uint8) {
	if maxHP <= 0 {
		return 80, 80, 90
	}
	ratio := float64(current) / float64(maxHP)
	if ratio > 0.5 {
		return 70, 210, 70
	} else if ratio > 0.25 {
		return 220, 200, 40
	}
	return 220, 60, 40
}

// drawStatBar draws a labeled stat bar with fill. Returns columns consumed.
func (e *Engine) drawStatBar(row, col int, label string, current, maximum, barWidth int,
	labelR, labelG, labelB, fillR, fillG, fillB, bgR, bgG, bgB uint8) int {
	startCol := col

	// Label
	for _, r := range label {
		if col < e.width && row >= 0 && row < e.height {
			e.next[row][col] = Cell{Ch: r, FgR: labelR, FgG: labelG, FgB: labelB,
				BgR: bgR, BgG: bgG, BgB: bgB, Bold: true}
		}
		col++
	}
	col++ // space

	// Bar
	filled := 0
	if maximum > 0 {
		filled = barWidth * current / maximum
	}
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	for i := 0; i < barWidth; i++ {
		x := col + i
		if x >= e.width || row < 0 || row >= e.height {
			break
		}
		if i < filled {
			e.next[row][x] = Cell{Ch: '\u2588', FgR: fillR, FgG: fillG, FgB: fillB,
				BgR: bgR, BgG: bgG, BgB: bgB}
		} else {
			e.next[row][x] = Cell{Ch: '\u2591', FgR: 45, FgG: 45, FgB: 55,
				BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}
	col += barWidth
	col++ // space

	// Numbers
	numText := fmt.Sprintf("%d/%d", current, maximum)
	for _, r := range numText {
		if col < e.width && row >= 0 && row < e.height {
			e.next[row][col] = Cell{Ch: r, FgR: 180, FgG: 180, FgB: 195,
				BgR: bgR, BgG: bgG, BgB: bgB}
		}
		col++
	}

	return col - startCol
}

// writeText writes colored text into a bounded region [col, maxCol). Returns the next column position.
func (e *Engine) writeText(row, col, maxCol int, text string, fgR, fgG, fgB, bgR, bgG, bgB uint8, bold bool) int {
	for _, r := range text {
		if col >= maxCol || col >= e.width {
			break
		}
		if row >= 0 && row < e.height && col >= 0 {
			e.next[row][col] = Cell{Ch: r, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: bold}
		}
		col++
	}
	return col
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

// renderDebugView draws a paginated debug view of pixel tile and player sprites.
// Uses the pixel buffer + collapse approach. Each tile is CharTileW x CharTileH on screen.
// Page 0: non-connected tile sprites, Page 1: connected tile sprites, Page 2: player sprites.
func (e *Engine) renderDebugView(viewerColor, page int, tick uint64) string {
	// Use the full screen as pixel buffer (no HUD in debug)
	debugPixH := e.height * 2
	debugPixW := e.width
	// Temporarily expand pixel buffer to cover the full screen
	savedH := e.pixBufH
	e.pixBufH = debugPixH
	if len(e.pixelBuf) < debugPixH {
		for len(e.pixelBuf) < debugPixH {
			e.pixelBuf = append(e.pixelBuf, make([]Pixel, debugPixW))
		}
	}
	for y := 0; y < debugPixH; y++ {
		if len(e.pixelBuf[y]) < debugPixW {
			e.pixelBuf[y] = make([]Pixel, debugPixW)
		}
	}

	// Clear with dark background
	bgR, bgG, bgB := uint8(18), uint8(18), uint8(24)
	for y := 0; y < debugPixH; y++ {
		for x := 0; x < debugPixW; x++ {
			e.pixelBuf[y][x] = P(bgR, bgG, bgB)
		}
	}

	pageNames := []string{"Tiles", "Connected/Blob", "Players"}
	if page < 0 || page >= len(pageNames) {
		page = 0
	}

	// Layout constants
	gap := 2
	charRowH := CharTileH + 2 // label row + tile char rows + gap row

	// Flow layout cursor (in char rows, not pixels)
	curX := 0
	curY := 2 // leave room for title

	placeGroup := func(label string, charWidth int) (int, int) {
		if curX > 0 && curX+charWidth > e.width {
			curX = 0
			curY += charRowH
		}
		sx, sy := curX, curY
		curX += charWidth + gap
		_ = label // label drawn after collapse
		return sx, sy
	}

	// Stamp pixel sprites at positions mapped from char to pixel coords
	stampAt := func(charX, charY int, sprite PixelSprite, transparent bool) {
		px := charX
		py := charY * 2 // 2 pixel rows per char row
		for row := 0; row < PixelTileH; row++ {
			bufY := py + row
			if bufY < 0 || bufY >= debugPixH {
				continue
			}
			for col := 0; col < PixelTileW; col++ {
				bufX := px + col
				if bufX < 0 || bufX >= debugPixW {
					continue
				}
				p := sprite[row][col]
				if transparent && p.Transparent {
					continue
				}
				e.pixelBuf[bufY][bufX] = p
			}
		}
	}

	type labelInfo struct {
		row, col int
		text     string
	}
	var labels []labelInfo

	switch page {
	case 0: // Simple tile sprites (non-connected, non-blob)
		for _, name := range pixelTileNames(e.sprites) {
			if e.sprites.TileIsConnected(name) || e.sprites.TileIsBlob(name) {
				continue
			}

			ts := e.sprites.GetTileSprites(name, tick)
			maxDY := 0
			for _, ov := range ts.Overlays {
				if ov.DY > maxDY {
					maxDY = ov.DY
				}
			}
			overlayCharRows := maxDY * CharTileH

			sx, sy := placeGroup(name, CharTileW)
			labels = append(labels, labelInfo{sy, sx, name})

			baseY := sy + 1 + overlayCharRows
			stampAt(sx, baseY, ts.Base, false)
			for _, ov := range ts.Overlays {
				stampAt(sx, baseY-ov.DY*CharTileH, ov.Sprite, true)
			}

			if overlayCharRows > 0 {
				curY += overlayCharRows
			}
		}

	case 1: // Connected + Blob tile sprites — practical preview
		connPattern := [][]bool{
			{false, true, true, true, true, true, false},
			{false, true, false, true, false, true, false},
			{false, true, true, true, true, true, false},
			{false, false, false, true, false, false, false},
			{false, false, false, true, false, false, false},
		}
		connPatH := len(connPattern)
		connPatW := len(connPattern[0])

		connPatMask := func(px, py int) uint8 {
			var mask uint8
			if py > 0 && connPattern[py-1][px] {
				mask |= ConnN
			}
			if px+1 < connPatW && connPattern[py][px+1] {
				mask |= ConnE
			}
			if py+1 < connPatH && connPattern[py+1][px] {
				mask |= ConnS
			}
			if px > 0 && connPattern[py][px-1] {
				mask |= ConnW
			}
			return mask
		}

		for _, name := range pixelTileNames(e.sprites) {
			if !e.sprites.TileIsConnected(name) {
				continue
			}

			// Sample sprite
			sx, sy := placeGroup(name, CharTileW)
			labels = append(labels, labelInfo{sy, sx, name})
			sprite := e.sprites.GetConnectedTileSprite(name, ConnE|ConnW)
			stampAt(sx, sy+1, sprite, false)

			// Practical preview grid
			curX = 0
			curY += charRowH
			labels = append(labels, labelInfo{curY, curX, "preview"})
			gridY := curY + 1

			for py := 0; py < connPatH; py++ {
				for px := 0; px < connPatW; px++ {
					screenX := curX + px*CharTileW
					screenY := gridY + py*CharTileH
					if connPattern[py][px] {
						mask := connPatMask(px, py)
						sprite := e.sprites.GetConnectedTileSprite(name, mask)
						stampAt(screenX, screenY, sprite, false)
					} else {
						grassTS := e.sprites.GetTileSprites("grass", tick)
						stampAt(screenX, screenY, grassTS.Base, false)
					}
				}
			}

			curY = gridY + connPatH*CharTileH + 1
			curX = 0
		}

		// Blob tiles — 8-neighbor-aware preview
		blobPattern := [][]bool{
			{false, false, true, true, true, false, false},
			{false, true, true, true, true, true, false},
			{false, true, true, true, true, true, false},
			{false, true, true, false, true, true, false},
			{false, true, true, true, true, true, false},
			{false, false, true, true, true, false, false},
		}
		blobPatH := len(blobPattern)
		blobPatW := len(blobPattern[0])

		blobPatMask := func(px, py int) uint8 {
			check := func(dx, dy int) bool {
				nx, ny := px+dx, py+dy
				if nx < 0 || nx >= blobPatW || ny < 0 || ny >= blobPatH {
					return false
				}
				return blobPattern[ny][nx]
			}

			var mask uint8
			n := check(0, -1)
			e := check(1, 0)
			s := check(0, 1)
			w := check(-1, 0)

			if n {
				mask |= BlobN
			}
			if e {
				mask |= BlobE
			}
			if s {
				mask |= BlobS
			}
			if w {
				mask |= BlobW
			}
			if n && e && check(1, -1) {
				mask |= BlobNE
			}
			if s && e && check(1, 1) {
				mask |= BlobSE
			}
			if s && w && check(-1, 1) {
				mask |= BlobSW
			}
			if n && w && check(-1, -1) {
				mask |= BlobNW
			}
			return mask
		}

		for _, name := range pixelTileNames(e.sprites) {
			if !e.sprites.TileIsBlob(name) {
				continue
			}

			// Sample sprite (center)
			sx, sy := placeGroup(name, CharTileW)
			isBorder := e.sprites.TileIsBorderBlob(name)
			label := name
			if isBorder {
				label = name + " (border)"
			}
			labels = append(labels, labelInfo{sy, sx, label})
			sprite := e.sprites.GetBlobTileSprite(name, 0xFF) // all neighbors = center
			stampAt(sx, sy+1, sprite, false)

			// Practical preview grid
			curX = 0
			curY += charRowH
			labels = append(labels, labelInfo{curY, curX, "preview"})
			gridY := curY + 1

			for py := 0; py < blobPatH; py++ {
				for px := 0; px < blobPatW; px++ {
					screenX := curX + px*CharTileW
					screenY := gridY + py*CharTileH
					if blobPattern[py][px] {
						if isBorder {
							// Border blob: tile itself renders as center
							center := e.sprites.GetBlobTileSprite(name, 0xFF)
							stampAt(screenX, screenY, center, false)
						} else {
							mask := blobPatMask(px, py)
							sprite := e.sprites.GetBlobTileSprite(name, mask)
							stampAt(screenX, screenY, sprite, false)
						}
					} else {
						if isBorder {
							// Border blob: neighbor renders transition
							mask := blobPatMask(px, py)
							if mask != 0 {
								sprite := e.sprites.GetBorderBlobTileSprite(name, mask)
								stampAt(screenX, screenY, sprite, false)
							} else {
								grassTS := e.sprites.GetTileSprites("grass", tick)
								stampAt(screenX, screenY, grassTS.Base, false)
							}
						} else {
							grassTS := e.sprites.GetTileSprites("grass", tick)
							stampAt(screenX, screenY, grassTS.Base, false)
						}
					}
				}
			}

			curY = gridY + blobPatH*CharTileH + 1
			curX = 0
		}

	case 2: // Player sprites
		dirNames := []string{"down", "up", "left", "right"}
		for i, dName := range dirNames {
			sx, sy := placeGroup(dName, CharTileW)
			labels = append(labels, labelInfo{sy, sx, dName})
			sprite := e.sprites.GetPlayerSprite(i, viewerColor)
			stampAt(sx, sy+1, sprite, true)
		}
	}

	// Collapse pixel buffer into next[][] (full screen, not just world area)
	for row := 0; row < e.height; row++ {
		topPixRow := row * 2
		botPixRow := row*2 + 1
		for col := 0; col < e.width; col++ {
			if col >= debugPixW {
				break
			}
			var top, bot Pixel
			if topPixRow < debugPixH {
				top = e.pixelBuf[topPixRow][col]
			}
			if botPixRow < debugPixH {
				bot = e.pixelBuf[botPixRow][col]
			}
			e.next[row][col] = Cell{
				Ch:  '▄',
				FgR: bot.R, FgG: bot.G, FgB: bot.B,
				BgR: top.R, BgG: top.G, BgB: top.B,
			}
		}
	}

	// Restore pixel buffer height
	e.pixBufH = savedH

	// Draw title and labels on top of collapsed cells (character-based)
	title := fmt.Sprintf("SPRITE DEBUG [%d/%d: %s] (\u2190\u2192 nav, ~ close)", page+1, len(pageNames), pageNames[page])
	for i, r := range []rune(title) {
		if i+1 < e.width {
			e.next[0][i+1] = Cell{Ch: r, FgR: 255, FgG: 220, FgB: 100, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true}
		}
	}
	for _, l := range labels {
		for i, r := range []rune(l.text) {
			x := l.col + i
			if x >= 0 && x < e.width && l.row >= 0 && l.row < e.height {
				e.next[l.row][x] = Cell{Ch: r, FgR: 160, FgG: 160, FgB: 175, BgR: bgR, BgG: bgG, BgB: bgB}
			}
		}
	}

	return e.emitDiff()
}

// pixelTileNames returns the tile names in display order, filtered to those in the registry.
func pixelTileNames(reg *SpriteRegistry) []string {
	var names []string
	for _, name := range tileNameOrder {
		if reg.HasTile(name) {
			names = append(names, name)
		}
	}
	return names
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
