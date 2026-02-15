package game

import (
	"fmt"
	"sync"
	"time"

	"happy-place-2/internal/maps"
)

const (
	InputChanSize = 256
)

// WorldState holds world-wide info shared across all maps.
type WorldState struct {
	TotalPlayers int
	Tick         uint64
}

// MapState holds the state for a single map sent to a session.
type MapState struct {
	Map     *maps.Map
	Players []PlayerSnapshot
}

// GameState is a snapshot sent to each session for rendering.
type GameState struct {
	World WorldState
	Map   MapState
}

// RenderChan is the per-session channel that receives game state snapshots.
type RenderChan chan GameState

// savedState holds persisted player data for reconnecting players.
type savedState struct {
	X, Y    int
	Color   int
	MapName string
}

// GameLoop is the central game loop singleton.
type GameLoop struct {
	world   *World
	inputCh chan InputEvent
	tickCount uint64

	mu          sync.RWMutex
	players     map[string]*Player
	renderChans map[string]RenderChan
	saved       map[string]savedState // keyed by username

	stopCh chan struct{}
}

// NewGameLoop creates and returns a new game loop.
func NewGameLoop(world *World) *GameLoop {
	return &GameLoop{
		world:       world,
		inputCh:     make(chan InputEvent, InputChanSize),
		players:     make(map[string]*Player),
		renderChans: make(map[string]RenderChan),
		saved:       make(map[string]savedState),
		stopCh:      make(chan struct{}),
	}
}

// InputChan returns the shared input channel for sessions to send events.
func (gl *GameLoop) InputChan() chan<- InputEvent {
	return gl.inputCh
}

// AddPlayer registers a player using their username as identity.
// If the username was seen before, position, color, and map are restored.
// Returns the effective player ID and the render channel.
func (gl *GameLoop) AddPlayer(name string) (string, RenderChan) {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	// If this username is already online, add a suffix
	id := name
	if _, online := gl.players[id]; online {
		id = fmt.Sprintf("%s_%04d", name, time.Now().UnixNano()%10000)
	}

	var player *Player
	if ss, ok := gl.saved[name]; ok {
		// Validate saved map still exists, fall back to default
		mapName := ss.MapName
		if gl.world.GetMap(mapName) == nil {
			mapName, ss.X, ss.Y = gl.world.SpawnPoint()
		}
		player = &Player{
			ID:      id,
			Name:    name,
			X:       ss.X,
			Y:       ss.Y,
			Color:   ss.Color,
			MapName: mapName,
		}
	} else {
		// Brand new player
		mapName, spawnX, spawnY := gl.world.SpawnPoint()
		player = &Player{
			ID:      id,
			Name:    name,
			X:       spawnX,
			Y:       spawnY,
			Color:   NextPlayerColor(),
			MapName: mapName,
		}
	}

	gl.players[id] = player
	ch := make(RenderChan, 2)
	gl.renderChans[id] = ch
	return id, ch
}

// RemovePlayer saves the player's state and unregisters them.
func (gl *GameLoop) RemovePlayer(id string) {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	if p, ok := gl.players[id]; ok {
		gl.saved[p.Name] = savedState{X: p.X, Y: p.Y, Color: p.Color, MapName: p.MapName}
		delete(gl.players, id)
	}
	if ch, ok := gl.renderChans[id]; ok {
		close(ch)
		delete(gl.renderChans, id)
	}
}

// Run starts the game loop. Blocks until Stop is called.
func (gl *GameLoop) Run() {
	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()

	for {
		select {
		case <-gl.stopCh:
			return
		case <-ticker.C:
			gl.tick()
		}
	}
}

// Stop shuts down the game loop.
func (gl *GameLoop) Stop() {
	close(gl.stopCh)
}

func (gl *GameLoop) tick() {
	// Drain all pending input events
	for {
		select {
		case ev := <-gl.inputCh:
			gl.processInput(ev)
		default:
			goto drained
		}
	}
drained:

	gl.tickCount++

	// Update animations for all players
	gl.mu.RLock()
	for _, p := range gl.players {
		updatePlayerAnimation(p)
	}
	gl.mu.RUnlock()

	// Build per-player snapshots grouped by map
	gl.mu.RLock()
	totalPlayers := len(gl.players)

	// Group player snapshots by map name
	byMap := make(map[string][]PlayerSnapshot)
	for _, p := range gl.players {
		byMap[p.MapName] = append(byMap[p.MapName], p.Snapshot())
	}

	ws := WorldState{
		TotalPlayers: totalPlayers,
		Tick:         gl.tickCount,
	}

	// Send each player a GameState with only their map's players
	for id, ch := range gl.renderChans {
		p := gl.players[id]
		m := gl.world.GetMap(p.MapName)
		state := GameState{
			World: ws,
			Map: MapState{
				Map:     m,
				Players: byMap[p.MapName],
			},
		}
		select {
		case ch <- state:
		default:
			// Drop frame for slow client
		}
	}
	gl.mu.RUnlock()
}

// updatePlayerAnimation advances animation state each tick.
func updatePlayerAnimation(p *Player) {
	// Decrement move cooldown
	if p.MoveCooldown > 0 {
		p.MoveCooldown--
	}

	// Advance animation tick
	p.AnimTick++

	if p.Anim == AnimWalking {
		p.AnimTimer--
		if p.AnimTimer <= 0 {
			// Walk animation finished, switch to idle
			p.Anim = AnimIdle
			p.AnimFrame = 0
			p.AnimTick = 0
		} else if p.AnimTick >= WalkFrameInterval {
			p.AnimFrame = (p.AnimFrame + 1) % 2
			p.AnimTick = 0
		}
	} else {
		// Idle animation
		if p.AnimTick >= IdleFrameInterval {
			p.AnimFrame = (p.AnimFrame + 1) % 2
			p.AnimTick = 0
		}
	}
}

func (gl *GameLoop) processInput(ev InputEvent) {
	gl.mu.RLock()
	player, ok := gl.players[ev.PlayerID]
	gl.mu.RUnlock()
	if !ok {
		return
	}

	// Toggle debug view
	if ev.Action == ActionDebug {
		player.DebugView = !player.DebugView
		return
	}

	// Debug page navigation (only when debug view is open)
	if player.DebugView {
		const debugPageCount = 3
		switch ev.Action {
		case ActionLeft:
			player.DebugPage = (player.DebugPage - 1 + debugPageCount) % debugPageCount
			return
		case ActionRight:
			player.DebugPage = (player.DebugPage + 1) % debugPageCount
			return
		case ActionDebugPage1:
			player.DebugPage = 0
			return
		case ActionDebugPage2:
			player.DebugPage = 1
			return
		case ActionDebugPage3:
			player.DebugPage = 2
			return
		default:
			return // ignore other actions in debug mode
		}
	}

	// Ignore page actions outside debug view
	if ev.Action == ActionDebugPage1 || ev.Action == ActionDebugPage2 || ev.Action == ActionDebugPage3 {
		return
	}

	// Determine desired facing direction
	var dir Direction
	switch ev.Action {
	case ActionUp:
		dir = DirUp
	case ActionDown:
		dir = DirDown
	case ActionLeft:
		dir = DirLeft
	case ActionRight:
		dir = DirRight
	default:
		return
	}

	// If facing a different direction, just turn (no move, no cooldown)
	if player.Dir != dir {
		player.Dir = dir
		return
	}

	// Already facing this direction — attempt movement
	if player.MoveCooldown > 0 {
		return
	}

	newX, newY := player.X, player.Y
	switch dir {
	case DirUp:
		newY--
	case DirDown:
		newY++
	case DirLeft:
		newX--
	case DirRight:
		newX++
	}

	canMove := gl.world.CanMoveTo(player.MapName, newX, newY)
	if canMove {
		player.X = newX
		player.Y = newY
		player.Anim = AnimWalking
		player.AnimTimer = WalkAnimDuration
		player.MoveCooldown = MoveRepeatDelay
		player.AnimTick = 0

		// Check for portal at new position
		portal := gl.world.PortalAt(player.MapName, newX, newY)
		if portal != nil {
			player.MapName = portal.TargetMap
			player.X = portal.TargetX
			player.Y = portal.TargetY
		}
	}
}
