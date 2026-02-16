package render

import (
	"fmt"
	"strings"

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

	vp := NewViewport(viewerX, viewerY, termW, termH, tileMap.Width, tileMap.Height, HUDRows)

	// Clear next buffer
	bgCell := Cell{Ch: ' ', BgR: 10, BgG: 10, BgB: 15}
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			e.next[y][x] = bgCell
		}
	}

	// --- Pass 1: Ground tiles + collect overlays ---
	const maxOverlayDY = 3 // scan extra rows below viewport for tiles whose overlays reach in
	signSprite := SignSprite()

	type pendingOverlay struct {
		sx, sy int
		sprite Sprite
	}
	var overlays []pendingOverlay

	scanH := vp.ViewH + maxOverlayDY
	for ty := 0; ty < scanH; ty++ {
		for tx := 0; tx < vp.ViewW; tx++ {
			wx := vp.CamX + tx
			wy := vp.CamY + ty
			if wx < 0 || wx >= tileMap.Width || wy < 0 || wy >= tileMap.Height {
				continue
			}
			tile := tileMap.TileAt(wx, wy)
			ts := TileSprite(tile, wx, wy, tick, tileMap)
			sx := vp.OffsetX + tx*TileWidth
			sy := vp.OffsetY + ty*TileHeight

			// Only stamp base for tiles within the visible viewport
			if ty < vp.ViewH {
				e.stampSprite(sx, sy, ts.Base, false)
				if tileMap.InteractionAt(wx, wy) != nil {
					e.stampSprite(sx, sy, signSprite, true)
				}
			}

			// Collect overlays — rendered after players
			for _, ov := range ts.Overlays {
				ovSY := sy - ov.DY*TileHeight
				overlays = append(overlays, pendingOverlay{sx: sx, sy: ovSY, sprite: ov.Sprite})
			}
		}
	}

	// --- Pass 2: Players ---
	var viewerPopup *InteractionPopup
	for _, p := range players {
		sx, sy := vp.WorldToScreen(p.X, p.Y)
		if sx+TileWidth <= 0 || sx >= termW || sy+TileHeight <= 0 || sy >= (termH-HUDRows) {
			continue
		}
		isSelf := p.ID == viewerID
		if isSelf && p.ActiveInteraction != nil {
			viewerPopup = p.ActiveInteraction
		}
		sprite := PlayerSprite(p.Dir, p.Anim, p.AnimFrame, p.Color, isSelf, p.Name)
		e.stampSprite(sx, sy, sprite, true)
	}

	// --- Pass 3: Overlays (on top of players) ---
	for _, ov := range overlays {
		e.stampSprite(ov.sx, ov.sy, ov.sprite, true)
	}

	// Draw interaction popup above sign tile
	if viewerPopup != nil {
		e.drawInteractionPopup(viewerPopup, vp, termH)
	}

	// Draw HUD
	e.drawHUD(viewerName, viewerColor, totalPlayers, tileMap.Name, statsInfo)

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

// --- Interaction Popup ---

func (e *Engine) drawInteractionPopup(popup *InteractionPopup, vp Viewport, termH int) {
	signSX, signSY := vp.WorldToScreen(popup.WorldX, popup.WorldY)

	textRunes := []rune(popup.Text)
	popupW := len(textRunes) + 4 // "│ " + text + " │"
	popupH := 3                  // top border, text, bottom border

	// Horizontal: center on sign tile, clamp to screen
	popupX := signSX + (TileWidth-popupW)/2
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
		popupY = signSY + TileHeight
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
	e.writeText(row3, 1, splitCol, "←↑↓→/WASD Move  │  Q Quit", 130, 130, 145, bgR, bgG, bgB, false)

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

// renderDebugView draws a paginated debug view of tile and player sprites.
// Page 0: non-connected tile sprites, Page 1: connected tile sprites, Page 2: player sprites.
func (e *Engine) renderDebugView(viewerColor, page int, tick uint64) string {
	// Clear buffer with dark background
	bgCell := Cell{Ch: ' ', BgR: 18, BgG: 18, BgB: 24}
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			e.next[y][x] = bgCell
		}
	}

	pageNames := []string{"Tiles", "Connected", "Players"}
	if page < 0 || page >= len(pageNames) {
		page = 0
	}

	// Title row
	title := fmt.Sprintf("SPRITE DEBUG [%d/%d: %s] (\u2190\u2192 nav, ~ close)", page+1, len(pageNames), pageNames[page])
	titleRunes := []rune(title)
	for i, r := range titleRunes {
		if i+1 < e.width {
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
	placeGroup := func(label string, width int) (int, int) {
		if curX > 0 && curX+width > e.width {
			curX = 0
			curY += rowHeight
		}
		sx, sy := curX, curY
		writeLabel(sy, sx, label)
		curX += width + gap*2
		return sx, sy
	}

	switch page {
	case 0: // Non-connected tile sprites with variants
		for i := range tileList {
			entry := &tileList[i]
			if entry.connected {
				continue
			}

			// Determine max overlay DY for height calculation
			maxDY := 0
			if entry.variants > 0 {
				wx, wy := variantCoord(0, entry.variants)
				ts := entry.fn(wx, wy, tick, nil)
				for _, ov := range ts.Overlays {
					if ov.DY > maxDY {
						maxDY = ov.DY
					}
				}
			}
			overlayPixels := maxDY * TileHeight

			groupWidth := entry.variants*TileWidth + (entry.variants-1)*gap
			sx, sy := placeGroup(entry.name, groupWidth)

			for v := 0; v < entry.variants; v++ {
				wx, wy := variantCoord(v, entry.variants)
				ts := entry.fn(wx, wy, tick, nil)
				baseX := sx + v*(TileWidth+gap)
				baseY := sy + 1 + overlayPixels // push base down so overlays fit above
				e.stampSprite(baseX, baseY, ts.Base, false)
				for _, ov := range ts.Overlays {
					e.stampSprite(baseX, baseY-ov.DY*TileHeight, ov.Sprite, true)
				}
			}

			// Advance cursor past the extra overlay height
			if overlayPixels > 0 {
				curY += overlayPixels
			}
		}

	case 1: // Connected tile sprites — practical preview
		// Pattern shows corners, runs, T-junction, and cross in context
		pattern := [][]bool{
			{false, true, true, true, true, true, false},
			{false, true, false, true, false, true, false},
			{false, true, true, true, true, true, false},
			{false, false, false, true, false, false, false},
			{false, false, false, true, false, false, false},
		}
		patH := len(pattern)
		patW := len(pattern[0])

		patMask := func(px, py int) uint8 {
			var mask uint8
			if py > 0 && pattern[py-1][px] {
				mask |= ConnN
			}
			if px+1 < patW && pattern[py][px+1] {
				mask |= ConnE
			}
			if py+1 < patH && pattern[py+1][px] {
				mask |= ConnS
			}
			if px > 0 && pattern[py][px-1] {
				mask |= ConnW
			}
			return mask
		}

		for i := range tileList {
			entry := &tileList[i]
			if !entry.connected {
				continue
			}

			// Variant row
			groupWidth := entry.variants*TileWidth + (entry.variants-1)*gap
			sx, sy := placeGroup(entry.name, groupWidth)
			for v := 0; v < entry.variants; v++ {
				sprite := entry.connFn(ConnE|ConnW, uint(v), tick)
				e.stampSprite(sx+v*(TileWidth+gap), sy+1, sprite, false)
			}

			// Practical preview grid with grass background
			curX = 0
			curY += rowHeight
			writeLabel(curY, curX, "preview")
			gridY := curY + 1

			for py := 0; py < patH; py++ {
				for px := 0; px < patW; px++ {
					screenX := curX + px*TileWidth
					screenY := gridY + py*TileHeight
					if pattern[py][px] {
						mask := patMask(px, py)
						v := TileHash(px, py) % uint(entry.variants)
						sprite := entry.connFn(mask, v, tick)
						e.stampSprite(screenX, screenY, sprite, false)
					} else {
						sprite := grassSprite(TileHash(px, py)%4, tick)
						e.stampSprite(screenX, screenY, sprite, false)
					}
				}
			}

			curY = gridY + patH*TileHeight + 1
			curX = 0
		}

	case 2: // Player sprites
		dirNames := []string{"down", "up", "left", "right"}
		for i, dName := range dirNames {
			sx, sy := placeGroup(dName, TileWidth)
			sprite := PlayerSprite(i, 0, 0, viewerColor, true, "Debug")
			e.stampSprite(sx, sy+1, sprite, true)
		}
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
