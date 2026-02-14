package game

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
	Color int // index into the render color palette
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

const numPlayerColors = 6

var colorIndex int

// NextPlayerColor returns the next color index from the rotating palette.
func NextPlayerColor() int {
	c := colorIndex % numPlayerColors
	colorIndex++
	return c
}
