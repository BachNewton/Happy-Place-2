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
}

// RenderChan is the per-session channel that receives game state snapshots.
type RenderChan chan GameState

// GameLoop is the central game loop singleton.
type GameLoop struct {
	world   *World
	inputCh chan InputEvent

	mu          sync.RWMutex
	players     map[string]*Player
	renderChans map[string]RenderChan

	stopCh chan struct{}
}

// NewGameLoop creates and returns a new game loop.
func NewGameLoop(world *World) *GameLoop {
	return &GameLoop{
		world:       world,
		inputCh:     make(chan InputEvent, InputChanSize),
		players:     make(map[string]*Player),
		renderChans: make(map[string]RenderChan),
		stopCh:      make(chan struct{}),
	}
}

// InputChan returns the shared input channel for sessions to send events.
func (gl *GameLoop) InputChan() chan<- InputEvent {
	return gl.inputCh
}

// AddPlayer registers a new player and returns their render channel.
func (gl *GameLoop) AddPlayer(id, name string) RenderChan {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	// Handle name collisions
	finalName := name
	for _, p := range gl.players {
		if p.Name == finalName {
			finalName = fmt.Sprintf("%s_%04d", name, time.Now().UnixNano()%10000)
			break
		}
	}

	spawnX, spawnY := gl.world.SpawnPoint()
	player := &Player{
		ID:    id,
		Name:  finalName,
		X:     spawnX,
		Y:     spawnY,
		Color: NextPlayerColor(),
	}
	gl.players[id] = player

	ch := make(RenderChan, 2)
	gl.renderChans[id] = ch
	return ch
}

// RemovePlayer unregisters a player.
func (gl *GameLoop) RemovePlayer(id string) {
	gl.mu.Lock()
	defer gl.mu.Unlock()

	delete(gl.players, id)
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

	// Build snapshot and broadcast
	gl.mu.RLock()
	state := GameState{
		Players: make([]PlayerSnapshot, 0, len(gl.players)),
		Map:     gl.world.Map,
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
