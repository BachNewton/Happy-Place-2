package game

import "happy-place-2/internal/render"

// Action represents a player input action.
type Action int

const (
	ActionNone Action = iota
	ActionUp
	ActionDown
	ActionLeft
	ActionRight
	ActionQuit
)

// InputEvent carries a player action into the game loop.
type InputEvent struct {
	PlayerID string
	Action   Action
}

// Player holds the game state for a connected player.
type Player struct {
	ID    string
	Name  string
	X, Y  int
	Color int
}

// PlayerSnapshot is a read-only copy of player state for rendering.
type PlayerSnapshot struct {
	ID    string
	Name  string
	X, Y  int
	Color int
}

// Snapshot returns a read-only copy of the player.
func (p *Player) Snapshot() PlayerSnapshot {
	return PlayerSnapshot{
		ID:    p.ID,
		Name:  p.Name,
		X:     p.X,
		Y:     p.Y,
		Color: p.Color,
	}
}

// DisplayChar returns the character to display for this player.
// Self sees '@', others see the first letter of their name.
func (ps PlayerSnapshot) DisplayChar(viewerID string) rune {
	if ps.ID == viewerID {
		return '@'
	}
	if len(ps.Name) > 0 {
		return rune(ps.Name[0])
	}
	return '?'
}

var colorIndex int

// NextPlayerColor returns the next color from the rotating palette.
func NextPlayerColor() int {
	c := render.PlayerColors[colorIndex%len(render.PlayerColors)]
	colorIndex++
	return c
}
