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

// Direction the player is facing.
type Direction int

const (
	DirDown  Direction = iota // default — face the camera
	DirUp
	DirLeft
	DirRight
)

// AnimState is the player's current animation state.
type AnimState int

const (
	AnimIdle    AnimState = iota
	AnimWalking
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

	Dir          Direction
	Anim         AnimState
	AnimFrame    int // current frame index
	AnimTimer    int // ticks remaining in walk state
	AnimTick     int // ticks since last frame advance
	MoveCooldown int // ticks until next move allowed
}

// PlayerSnapshot is a read-only copy of player state for rendering.
type PlayerSnapshot struct {
	ID        string
	Name      string
	X, Y      int
	Color     int
	Dir       Direction
	Anim      AnimState
	AnimFrame int
}

// Snapshot returns a read-only copy of the player.
func (p *Player) Snapshot() PlayerSnapshot {
	return PlayerSnapshot{
		ID:        p.ID,
		Name:      p.Name,
		X:         p.X,
		Y:         p.Y,
		Color:     p.Color,
		Dir:       p.Dir,
		Anim:      p.Anim,
		AnimFrame: p.AnimFrame,
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
