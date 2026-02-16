package render

import (
	"fmt"
	"strings"
)

// Combat phase constants mirroring game.CombatPhase values.
const (
	cPhaseTransition  = 0
	cPhasePlayerTurn  = 1
	cPhaseEnemyTurn   = 2
	cPhaseEnemyActing = 3
	cPhaseVictory     = 4
	cPhaseDefeat      = 5
)

// renderCombatView renders the full combat screen.
func (e *Engine) renderCombatView(combat *CombatRenderData, viewerName string, viewerColor, totalPlayers int, tick uint64, stats HUDStats) string {
	// Transition flash effect: fill screen with dark red/black
	if combat.Transitioning {
		flashR, flashG, flashB := uint8(40), uint8(5), uint8(5)
		pulse := uint8((tick / 3) % 2 * 15)
		flashR += pulse

		for y := 0; y < e.height; y++ {
			for x := 0; x < e.width; x++ {
				e.next[y][x] = Cell{Ch: ' ', BgR: flashR, BgG: flashG, BgB: flashB}
			}
		}
		// Center "ENCOUNTER!" text
		msg := "!! ENCOUNTER !!"
		msgRunes := []rune(msg)
		cy := e.height / 2
		cx := (e.width - len(msgRunes)) / 2
		for i, r := range msgRunes {
			if cx+i >= 0 && cx+i < e.width && cy >= 0 && cy < e.height {
				e.next[cy][cx+i] = Cell{Ch: r, FgR: 255, FgG: 60, FgB: 60, BgR: flashR, BgG: flashG, BgB: flashB, Bold: true}
			}
		}
		return e.emitDiff()
	}

	// Dark combat background
	bgR, bgG, bgB := uint8(12), uint8(12), uint8(18)
	for y := 0; y < e.height; y++ {
		for x := 0; x < e.width; x++ {
			e.next[y][x] = Cell{Ch: ' ', BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}

	hudY := e.height - HUDRows
	bR, bG, bB := uint8(100), uint8(70), uint8(55) // border color

	// ┌─────────────────────────┐  top border
	e.drawBoxRow(0, '┌', '─', '┐', bR, bG, bB, bgR, bgG, bgB)

	// │ side borders on all content rows
	for y := 1; y < hudY; y++ {
		if y >= 0 && y < e.height {
			e.next[y][0] = Cell{Ch: '│', FgR: bR, FgG: bG, FgB: bB, BgR: bgR, BgG: bgG, BgB: bgB}
			if e.width > 1 {
				e.next[y][e.width-1] = Cell{Ch: '│', FgR: bR, FgG: bG, FgB: bB, BgR: bgR, BgG: bgG, BgB: bgB}
			}
		}
	}

	curY := 1

	// --- Enemy area ---
	livingIdx := 0
	isViewerTurn := combat.Phase == cPhasePlayerTurn && combat.CurrentTurn == combat.ViewerID
	for _, enemy := range combat.Enemies {
		if curY+2 >= hudY-5 {
			break
		}
		targeted := false
		if isViewerTurn && enemy.Alive && combat.ViewerAction >= 1 && combat.ViewerAction <= 3 {
			if livingIdx == combat.ViewerTarget {
				targeted = true
			}
		}
		e.drawEnemyRow(curY, enemy, tick, targeted)
		if enemy.Alive {
			livingIdx++
		}
		curY += 2
	}

	// ├── BATTLE  Round N ──────┤  enemy/player divider
	sepText := fmt.Sprintf(" BATTLE  Round %d ", combat.Round)
	e.drawBoxDivider(curY, sepText, bR, bG, bB, 200, 180, 80, bgR, bgG, bgB)
	curY++

	// --- Player area ---
	for _, cp := range combat.Players {
		if curY+1 >= hudY-3 {
			break
		}
		e.drawCombatPlayerRow(curY, cp)
		curY++
	}

	// ├─────────────────────────┤  player/log divider
	e.drawBoxDivider(curY, "", bR, bG, bB, 0, 0, 0, bgR, bgG, bgB)
	curY++

	// --- Battle log ---
	logStart := hudY - len(combat.Log)
	if logStart < curY {
		logStart = curY
	}
	for i, msg := range combat.Log {
		row := logStart + i
		if row >= hudY {
			break
		}
		fgR, fgG, fgB := uint8(160), uint8(160), uint8(170)
		// Most recent message is brighter
		if i == len(combat.Log)-1 {
			fgR, fgG, fgB = 220, 220, 230
		}
		e.writeText(row, 2, e.width-1, msg, fgR, fgG, fgB, bgR, bgG, bgB, false)
	}

	// --- Victory/Defeat overlay ---
	if combat.Phase == cPhaseVictory {
		cy := e.height/2 - 1
		e.drawCenteredText(cy, "★ VICTORY ★", 255, 220, 50, bgR, bgG, bgB, true)
	} else if combat.Phase == cPhaseDefeat {
		cy := e.height/2 - 1
		e.drawCenteredText(cy, "✖ DEFEAT ✖", 255, 50, 50, bgR, bgG, bgB, true)
	}

	// --- Combat HUD (bottom rows) ---
	e.drawCombatHUD(combat, viewerName, viewerColor, totalPlayers, stats, bR, bG, bB)

	return e.emitDiff()
}

// drawBoxRow draws a full horizontal box line: left + fill + right.
func (e *Engine) drawBoxRow(row int, left, fill, right rune, fR, fG, fB, bR, bG, bB uint8) {
	if row < 0 || row >= e.height {
		return
	}
	e.next[row][0] = Cell{Ch: left, FgR: fR, FgG: fG, FgB: fB, BgR: bR, BgG: bG, BgB: bB}
	for x := 1; x < e.width-1; x++ {
		e.next[row][x] = Cell{Ch: fill, FgR: fR, FgG: fG, FgB: fB, BgR: bR, BgG: bG, BgB: bB}
	}
	if e.width > 1 {
		e.next[row][e.width-1] = Cell{Ch: right, FgR: fR, FgG: fG, FgB: fB, BgR: bR, BgG: bG, BgB: bB}
	}
}

// drawBoxDivider draws ├─ text ─┤ with optional centered text.
func (e *Engine) drawBoxDivider(row int, text string, fR, fG, fB, tR, tG, tB, bR, bG, bB uint8) {
	e.drawBoxRow(row, '├', '─', '┤', fR, fG, fB, bR, bG, bB)
	if text != "" {
		runes := []rune(text)
		cx := (e.width - len(runes)) / 2
		for i, r := range runes {
			x := cx + i
			if x > 0 && x < e.width-1 && row >= 0 && row < e.height {
				e.next[row][x] = Cell{Ch: r, FgR: tR, FgG: tG, FgB: tB, BgR: bR, BgG: bG, BgB: bB, Bold: true}
			}
		}
	}
}

// drawEnemyRow draws an enemy with name and HP bar.
func (e *Engine) drawEnemyRow(row int, enemy CombatEnemy, tick uint64, targeted bool) {
	bgR, bgG, bgB := uint8(12), uint8(12), uint8(18)

	// Target indicator (col 1, inside left border)
	if targeted {
		if 1 < e.width && row >= 0 && row < e.height {
			e.next[row][1] = Cell{Ch: '▶', FgR: 255, FgG: 220, FgB: 80, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true}
		}
	}
	col := 2
	if enemy.Alive {
		// Rat sprite chars
		ratChars := []rune{'>', '·', '~'}
		ratFrame := int(tick/8) % 2
		for i, ch := range ratChars {
			x := col + i
			if ratFrame == 1 && i == 2 {
				ch = '-'
			}
			if x < e.width-1 && row < e.height {
				e.next[row][x] = Cell{Ch: ch, FgR: 180, FgG: 160, FgB: 140, BgR: bgR, BgG: bgG, BgB: bgB}
			}
		}
		col += 4
	} else {
		col += 4
	}

	// Enemy name
	label := enemy.Label
	if !enemy.Alive {
		label += " (dead)"
	}
	nameR, nameG, nameB := uint8(200), uint8(160), uint8(140)
	if !enemy.Alive {
		nameR, nameG, nameB = 80, 80, 90
	}
	for i, r := range []rune(label) {
		x := col + i
		if x < e.width-1 && row < e.height {
			e.next[row][x] = Cell{Ch: r, FgR: nameR, FgG: nameG, FgB: nameB, BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}

	// HP bar on next row
	barRow := row + 1
	if barRow >= e.height {
		return
	}
	e.drawHPBar(barRow, 2, 20, enemy.HP, enemy.MaxHP, 200, 50, 50, enemy.Alive)
}

// drawHPBar draws a colored HP bar.
func (e *Engine) drawHPBar(row, col, width, hp, maxHP int, fgR, fgG, fgB uint8, alive bool) {
	bgR, bgG, bgB := uint8(12), uint8(12), uint8(18)

	// HP text
	hpText := fmt.Sprintf("HP %d/%d", hp, maxHP)
	for i, r := range []rune(hpText) {
		x := col + i
		if x < e.width-1 && row < e.height {
			e.next[row][x] = Cell{Ch: r, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}

	barStart := col + len([]rune(hpText)) + 1
	if !alive || maxHP <= 0 {
		return
	}

	filled := width * hp / maxHP
	if filled < 0 {
		filled = 0
	}

	for i := 0; i < width; i++ {
		x := barStart + i
		if x >= e.width-1 {
			break
		}
		ch := '░'
		r, g, b := uint8(40), uint8(40), uint8(50)
		if i < filled {
			ch = '█'
			// Color gradient: green > yellow > red
			ratio := float64(hp) / float64(maxHP)
			if ratio > 0.5 {
				r, g, b = 50, 200, 50
			} else if ratio > 0.25 {
				r, g, b = 220, 180, 30
			} else {
				r, g, b = 220, 50, 30
			}
		}
		e.next[row][x] = Cell{Ch: ch, FgR: r, FgG: g, FgB: b, BgR: bgR, BgG: bgG, BgB: bgB}
	}
}

// drawCombatPlayerRow draws a player's name and HP in the combat view.
func (e *Engine) drawCombatPlayerRow(row int, cp CombatPlayer) {
	bgR, bgG, bgB := uint8(12), uint8(12), uint8(18)
	col := 2

	// Player color indicator
	colorIdx := cp.Color % len(PlayerBGColors)
	pR, pG, pB := PlayerBGColors[colorIdx][0], PlayerBGColors[colorIdx][1], PlayerBGColors[colorIdx][2]

	if row < e.height && col < e.width-1 {
		e.next[row][col] = Cell{Ch: '●', FgR: pR, FgG: pG, FgB: pB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true}
	}
	col += 2

	// Player name
	name := cp.Name
	if !cp.Alive {
		name += " (fallen)"
	}
	if cp.IsViewer {
		name += " ←"
	}
	nameR, nameG, nameB := pR, pG, pB
	if !cp.Alive {
		nameR, nameG, nameB = 80, 80, 90
	}
	for i, r := range []rune(name) {
		x := col + i
		if x < e.width-1 {
			e.next[row][x] = Cell{Ch: r, FgR: nameR, FgG: nameG, FgB: nameB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: cp.IsViewer}
		}
	}

	// HP bar after name
	barCol := col + len([]rune(name)) + 2
	hpR, hpG, hpB := uint8(50), uint8(200), uint8(50)
	if !cp.Alive {
		hpR, hpG, hpB = 80, 80, 90
	}
	hpText := fmt.Sprintf("HP %d/%d", cp.HP, cp.MaxHP)
	for i, r := range []rune(hpText) {
		x := barCol + i
		if x < e.width-1 {
			e.next[row][x] = Cell{Ch: r, FgR: hpR, FgG: hpG, FgB: hpB, BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}
}

// drawCenteredText draws text centered on the given row.
func (e *Engine) drawCenteredText(row int, text string, fgR, fgG, fgB, bgR, bgG, bgB uint8, bold bool) {
	if row < 0 || row >= e.height {
		return
	}
	runes := []rune(text)
	cx := (e.width - len(runes)) / 2
	for i, r := range runes {
		x := cx + i
		if x >= 0 && x < e.width {
			e.next[row][x] = Cell{Ch: r, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: bold}
		}
	}
}

// drawCombatHUD draws the bottom 4 rows during combat with two-column layout.
// The separator row doubles as the bottom border of the combat box (┕━━━┙).
func (e *Engine) drawCombatHUD(combat *CombatRenderData, viewerName string, viewerColor, totalPlayers int, stats HUDStats, bdrR, bdrG, bdrB uint8) {
	hudY := e.height - HUDRows
	if hudY < 0 {
		return
	}

	splitCol := e.width / 2
	bgR, bgG, bgB := uint8(20), uint8(15), uint8(22)

	// Row 0: separator — bottom border of combat box with red-tinted gradient
	for x := 0; x < e.width; x++ {
		t := uint8(60 - x*40/max(e.width, 1))
		e.next[hudY][x] = Cell{
			Ch: '━', FgR: 140 + t, FgG: 40 + t, FgB: 40 + t,
			BgR: bgR, BgG: bgG, BgB: bgB,
		}
	}
	// Connect to side borders with corner characters
	e.next[hudY][0] = Cell{Ch: '┕', FgR: bdrR, FgG: bdrG, FgB: bdrB, BgR: bgR, BgG: bgG, BgB: bgB}
	if e.width > 1 {
		e.next[hudY][e.width-1] = Cell{Ch: '┙', FgR: bdrR, FgG: bdrG, FgB: bdrB, BgR: bgR, BgG: bgG, BgB: bgB}
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
			e.next[y][splitCol] = Cell{Ch: '│', FgR: 70, FgG: 40, FgB: 50, BgR: bgR, BgG: bgG, BgB: bgB}
		}
	}

	row1 := hudY + 1
	row2 := hudY + 2
	row3 := hudY + 3

	// Check if viewer is alive
	viewerAlive := true
	for _, cp := range combat.Players {
		if cp.IsViewer && !cp.Alive {
			viewerAlive = false
			break
		}
	}

	// --- Left column ---
	// Row 1: turn info
	var turnInfo string
	switch combat.Phase {
	case cPhasePlayerTurn:
		timerSec := combat.TurnTimer / 20
		if combat.CurrentTurn == combat.ViewerID {
			turnInfo = fmt.Sprintf("YOUR TURN  Round %d  [%ds]", combat.Round, timerSec)
		} else {
			turnName := combat.CurrentTurn
			for _, cp := range combat.Players {
				if cp.ID == combat.CurrentTurn {
					turnName = cp.Name
					break
				}
			}
			turnInfo = fmt.Sprintf("%s's turn  Round %d  [%ds]", turnName, combat.Round, timerSec)
		}
	case cPhaseEnemyTurn, cPhaseEnemyActing:
		turnInfo = fmt.Sprintf("Enemy turn  Round %d", combat.Round)
	case cPhaseVictory:
		turnInfo = "VICTORY!"
	case cPhaseDefeat:
		turnInfo = "DEFEAT..."
	default:
		turnInfo = "Preparing..."
	}
	e.writeText(row1, 1, splitCol, turnInfo, 220, 200, 180, bgR, bgG, bgB, false)

	// Row 2-3: actions/status
	if !viewerAlive {
		e.writeText(row2, 1, splitCol, "SPECTATING", 120, 120, 135, bgR, bgG, bgB, false)
	} else if combat.Phase == cPhaseVictory || combat.Phase == cPhaseDefeat {
		e.writeText(row2, 1, splitCol, "Returning to overworld...", 140, 140, 155, bgR, bgG, bgB, false)
	} else if combat.CurrentTurn != combat.ViewerID {
		e.writeText(row2, 1, splitCol, "WAITING...", 120, 120, 135, bgR, bgG, bgB, false)
	} else {
		// Action labels with highlight on selected action
		type actionLabel struct {
			key  string
			name string
			idx  int
		}
		actions := []actionLabel{
			{"1", "Melee", 1},
			{"2", "Ranged", 2},
			{"3", "Magic", 3},
			{"4", "Defend", 4},
		}
		col := 1
		for i, a := range actions {
			if i > 0 {
				col = e.writeText(row2, col, splitCol, " ", 80, 80, 95, bgR, bgG, bgB, false)
			}
			selected := combat.ViewerAction == a.idx
			label := a.key + ":" + a.name
			if selected {
				col = e.writeText(row2, col, splitCol, label, 255, 255, 220, bgR, bgG, bgB, true)
			} else {
				col = e.writeText(row2, col, splitCol, label, 120, 120, 135, bgR, bgG, bgB, false)
			}
		}
		if combat.ViewerAction >= 1 && combat.ViewerAction <= 3 {
			e.writeText(row3, 1, splitCol, "←→:Target  Enter:Confirm", 180, 180, 195, bgR, bgG, bgB, false)
		} else {
			e.writeText(row3, 1, splitCol, "Pick an action (1-4)", 130, 130, 145, bgR, bgG, bgB, false)
		}
	}

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

// emitDiff performs the buffer diff and produces ANSI output.
func (e *Engine) emitDiff() string {
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
