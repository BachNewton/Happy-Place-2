package render

import "fmt"

// ANSI escape code helpers

const (
	ESC   = "\x1b"
	CSI   = ESC + "["
	Reset = CSI + "0m"
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

// FgColor returns an SGR sequence for the given ANSI color code.
func FgColor(code int) string {
	return fmt.Sprintf("%s%dm", CSI, code)
}

// BgColor returns an SGR sequence for the given ANSI background color code.
func BgColor(code int) string {
	return fmt.Sprintf("%s%dm", CSI, code+10)
}

// ColoredChar returns a character with foreground and optional background color.
func ColoredChar(ch rune, fg, bg int) string {
	if bg > 0 {
		return fmt.Sprintf("%s%s%c%s", FgColor(fg), BgColor(bg), ch, Reset)
	}
	return fmt.Sprintf("%s%c%s", FgColor(fg), ch, Reset)
}

// Named color to ANSI code mapping.
var ColorNames = map[string]int{
	"black":   30,
	"red":     31,
	"green":   32,
	"yellow":  33,
	"blue":    34,
	"magenta": 35,
	"cyan":    36,
	"white":   37,
	"gray":    90,
	"grey":    90,
	"bright_red":     91,
	"bright_green":   92,
	"bright_yellow":  93,
	"bright_blue":    94,
	"bright_magenta": 95,
	"bright_cyan":    96,
	"bright_white":   97,
}

// PlayerColors is the rotating palette for player display.
var PlayerColors = []int{91, 92, 93, 94, 95, 96}

// ResolveColor converts a color name to an ANSI code. Defaults to white.
func ResolveColor(name string) int {
	if code, ok := ColorNames[name]; ok {
		return code
	}
	return 37 // white
}

// EnableAltScreen switches to the alternate screen buffer.
func EnableAltScreen() string {
	return CSI + "?1049h"
}

// DisableAltScreen switches back from the alternate screen buffer.
func DisableAltScreen() string {
	return CSI + "?1049l"
}
