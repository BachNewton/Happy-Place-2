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
	ActionDebug
	ActionDebugPage1
	ActionDebugPage2
	ActionDebugPage3
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

// ActiveInteraction represents a currently triggered interaction popup.
type ActiveInteraction struct {
	WorldX, WorldY int
	Text           string
}

// InputEvent carries a player action into the game loop.
type InputEvent struct {
	PlayerID string
	Action   Action
}

// Player holds the game state for a connected player.
type Player struct {
	ID      string
	Name    string
	X, Y    int
	Color   int // index into the render color palette
	MapName string

	Dir          Direction
	Anim         AnimState
	AnimFrame    int // current frame index
	AnimTimer    int // ticks remaining in walk state
	AnimTick     int // ticks since last frame advance
	MoveCooldown      int // ticks until next move allowed
	DebugView         bool
	DebugPage         int
	ActiveInteraction *ActiveInteraction
}

// PlayerSnapshot is a read-only copy of player state for rendering.
type PlayerSnapshot struct {
	ID                string
	Name              string
	X, Y              int
	Color             int
	MapName           string
	Dir               Direction
	Anim              AnimState
	AnimFrame         int
	DebugView         bool
	DebugPage         int
	ActiveInteraction *ActiveInteraction
}

// Snapshot returns a read-only copy of the player.
func (p *Player) Snapshot() PlayerSnapshot {
	return PlayerSnapshot{
		ID:                p.ID,
		Name:              p.Name,
		X:                 p.X,
		Y:                 p.Y,
		Color:             p.Color,
		MapName:           p.MapName,
		Dir:               p.Dir,
		Anim:              p.Anim,
		AnimFrame:         p.AnimFrame,
		DebugView:         p.DebugView,
		DebugPage:         p.DebugPage,
		ActiveInteraction: p.ActiveInteraction,
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
