package game

import (
	"fmt"
	"sync"
	"time"

	"happy-place-2/internal/maps"
)

const (
	TickRate      = 20 // ticks per second
	InputChanSize = 256
)

// GameState is a snapshot sent to each session for rendering.
type GameState struct {
	Players []PlayerSnapshot
	Map     *maps.Map
	Tick    uint64
}

// RenderChan is the per-session channel that receives game state snapshots.
type RenderChan chan GameState

// savedState holds persisted player data for reconnecting players.
type savedState struct {
	X, Y  int
	Color int
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
// If the username was seen before, position and color are restored.
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
		// Restore saved state
		player = &Player{
			ID:    id,
			Name:  name,
			X:     ss.X,
			Y:     ss.Y,
			Color: ss.Color,
		}
	} else {
		// Brand new player
		spawnX, spawnY := gl.world.SpawnPoint()
		player = &Player{
			ID:    id,
			Name:  name,
			X:     spawnX,
			Y:     spawnY,
			Color: NextPlayerColor(),
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
		gl.saved[p.Name] = savedState{X: p.X, Y: p.Y, Color: p.Color}
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

	// Build snapshot and broadcast
	gl.mu.RLock()
	state := GameState{
		Players: make([]PlayerSnapshot, 0, len(gl.players)),
		Map:     gl.world.Map,
		Tick:    gl.tickCount,
	}
	for _, p := range gl.players {
		state.Players = append(state.Players, p.Snapshot())
	}

	// Non-blocking send to each render channel
	for _, ch := range gl.renderChans {
		select {
		case ch <- state:
		default:
			// Drop frame for slow client
		}
	}
	gl.mu.RUnlock()
}

func (gl *GameLoop) processInput(ev InputEvent) {
	gl.mu.RLock()
	player, ok := gl.players[ev.PlayerID]
	gl.mu.RUnlock()
	if !ok {
		return
	}

	newX, newY := player.X, player.Y
	switch ev.Action {
	case ActionUp:
		newY--
	case ActionDown:
		newY++
	case ActionLeft:
		newX--
	case ActionRight:
		newX++
	default:
		return
	}

	if gl.world.CanMoveTo(newX, newY) {
		player.X = newX
		player.Y = newY
	}
}
