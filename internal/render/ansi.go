package render

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	ESC   = "\x1b"
	CSI   = ESC + "["
	Reset = CSI + "0m"

	// Synchronized output (DEC private mode 2026).
	// Tells the terminal to buffer all output and render atomically,
	// preventing tearing when large portions of the screen change.
	SyncStart = CSI + "?2026h"
	SyncEnd   = CSI + "?2026l"
)

// MoveTo positions the cursor at row, col (1-based).
func MoveTo(row, col int) string {
	return fmt.Sprintf("%s%d;%dH", CSI, row, col)
}

// ClearScreen clears the entire screen.
func ClearScreen() string {
	return CSI + "2J"
}

// HideCursor hides the terminal cursor.
func HideCursor() string {
	return CSI + "?25l"
}

// ShowCursor shows the terminal cursor.
func ShowCursor() string {
	return CSI + "?25h"
}

// EnableAltScreen switches to the alternate screen buffer.
func EnableAltScreen() string {
	return CSI + "?1049h"
}

// DisableAltScreen switches back from the alternate screen buffer.
func DisableAltScreen() string {
	return CSI + "?1049l"
}

// PlayerColors is the rotating palette index for player display.
var PlayerColors = []int{0, 1, 2, 3, 4, 5}

// PlayerFGColors are bright RGB foregrounds for player characters.
var PlayerFGColors = [][3]uint8{
	{255, 255, 255},
	{255, 255, 255},
	{255, 255, 255},
	{255, 255, 255},
	{255, 255, 255},
	{255, 255, 255},
}

// PlayerBGColors are the shirt colors for player characters.
var PlayerBGColors = [][3]uint8{
	{200, 60, 60},   // crimson red
	{55, 175, 70},   // forest green
	{65, 105, 225},  // royal blue
	{200, 170, 50},  // golden yellow
	{170, 60, 180},  // violet
	{45, 175, 175},  // teal
}

// WriteCellSGR writes a single cell's full SGR + character to the builder.
// Uses combined SGR to avoid state leakage between cells.
func WriteCellSGR(sb *strings.Builder, c Cell) {
	if c.Bold {
		sb.WriteString("\x1b[0;1;38;2;")
	} else {
		sb.WriteString("\x1b[0;38;2;")
	}
	sb.WriteString(strconv.Itoa(int(c.FgR)))
	sb.WriteByte(';')
	sb.WriteString(strconv.Itoa(int(c.FgG)))
	sb.WriteByte(';')
	sb.WriteString(strconv.Itoa(int(c.FgB)))
	sb.WriteString(";48;2;")
	sb.WriteString(strconv.Itoa(int(c.BgR)))
	sb.WriteByte(';')
	sb.WriteString(strconv.Itoa(int(c.BgG)))
	sb.WriteByte(';')
	sb.WriteString(strconv.Itoa(int(c.BgB)))
	sb.WriteByte('m')
	sb.WriteRune(c.Ch)
}

// AnsiToRGB converts a basic ANSI color code to RGB.
func AnsiToRGB(code int) (uint8, uint8, uint8) {
	switch code {
	case 30:
		return 0, 0, 0
	case 31:
		return 170, 0, 0
	case 32:
		return 0, 170, 0
	case 33:
		return 170, 170, 0
	case 34:
		return 0, 0, 170
	case 35:
		return 170, 0, 170
	case 36:
		return 0, 170, 170
	case 37:
		return 170, 170, 170
	case 90:
		return 85, 85, 85
	case 91:
		return 255, 85, 85
	case 92:
		return 85, 255, 85
	case 93:
		return 255, 255, 85
	case 94:
		return 85, 85, 255
	case 95:
		return 255, 85, 255
	case 96:
		return 85, 255, 255
	case 97:
		return 255, 255, 255
	default:
		return 170, 170, 170
	}
}
