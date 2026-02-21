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
	ActionConfirm
	ActionDefend
	ActionDebugCombat
	ActionDebugTileOverlay
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
	SlideTicksLeft    int // ticks remaining in slide interpolation
	SlideDirX         int // movement direction X (-1, 0, +1)
	SlideDirY         int // movement direction Y (-1, 0, +1)
	DebugView         bool
	DebugPage         int
	DebugTileOverlay  bool
	ActiveInteraction *ActiveInteraction

	// Stats
	HP, MaxHP           int
	Stamina, MaxStamina int
	MP, MaxMP           int
	Attack, Defense     int
	EXP                 int

	// Combat state
	FightID          int  // 0 = not in combat
	CombatTransition int  // ticks remaining in transition effect
	Defending        bool // halves incoming damage this round
	Dead             bool // dead in current fight (spectating)
	CombatAction     int  // selected action index (1-4)
	CombatTarget     int  // selected enemy target index
}

// DefaultHP is the starting/max HP for new players.
const DefaultHP = 30

// DefaultStamina is the starting/max stamina for new players.
const DefaultStamina = 20

// DefaultMP is the starting/max MP for new players.
const DefaultMP = 10

// DefaultAttack is the starting attack stat.
const DefaultAttack = 6

// DefaultDefense is the starting defense stat.
const DefaultDefense = 3

// Level returns the player's level derived from EXP.
func (p *Player) Level() int {
	return p.EXP/50 + 1
}

// InitStats sets default stats for a new player.
func (p *Player) InitStats() {
	p.HP = DefaultHP
	p.MaxHP = DefaultHP
	p.Stamina = DefaultStamina
	p.MaxStamina = DefaultStamina
	p.MP = DefaultMP
	p.MaxMP = DefaultMP
	p.Attack = DefaultAttack
	p.Defense = DefaultDefense
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
	DebugTileOverlay  bool
	ActiveInteraction *ActiveInteraction

	HP, MaxHP           int
	Stamina, MaxStamina int
	MP, MaxMP           int
	EXP                 int
	Level               int
	SlideOffsetX, SlideOffsetY int
	FightID                    int
	CombatTransition           int
	Dead                       bool
}

// Snapshot returns a read-only copy of the player.
func (p *Player) Snapshot() PlayerSnapshot {
	var slideX, slideY int
	if p.SlideTicksLeft > 0 && MoveRepeatDelay > 0 {
		slideX = -p.SlideDirX * SlideTilePixels * p.SlideTicksLeft / MoveRepeatDelay
		slideY = -p.SlideDirY * SlideTilePixels * p.SlideTicksLeft / MoveRepeatDelay
	}
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
		DebugTileOverlay:  p.DebugTileOverlay,
		ActiveInteraction: p.ActiveInteraction,
		HP:                p.HP,
		MaxHP:             p.MaxHP,
		Stamina:           p.Stamina,
		MaxStamina:        p.MaxStamina,
		MP:                p.MP,
		MaxMP:             p.MaxMP,
		EXP:               p.EXP,
		Level:             p.Level(),
		SlideOffsetX:      slideX,
		SlideOffsetY:      slideY,
		FightID:           p.FightID,
		CombatTransition:  p.CombatTransition,
		Dead:              p.Dead,
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
