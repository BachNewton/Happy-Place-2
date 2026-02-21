package game

import (
	"fmt"
	"math/rand"
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
	World  WorldState
	Map    MapState
	Combat *CombatState // non-nil when the viewer is in combat
}

// RenderChan is the per-session channel that receives game state snapshots.
type RenderChan chan GameState

// savedState holds persisted player data for reconnecting players.
type savedState struct {
	X, Y    int
	Color   int
	MapName string

	HP, MaxHP           int
	Stamina, MaxStamina int
	MP, MaxMP           int
	Attack, Defense     int
	EXP                 int
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

	fights      map[int]*Fight
	nextFightID int

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
		fights:      make(map[int]*Fight),
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
		player.HP = ss.HP
		player.MaxHP = ss.MaxHP
		player.Stamina = ss.Stamina
		player.MaxStamina = ss.MaxStamina
		player.MP = ss.MP
		player.MaxMP = ss.MaxMP
		player.Attack = ss.Attack
		player.Defense = ss.Defense
		player.EXP = ss.EXP
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
		player.InitStats()
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
		gl.saved[p.Name] = savedState{
			X: p.X, Y: p.Y, Color: p.Color, MapName: p.MapName,
			HP: p.HP, MaxHP: p.MaxHP,
			Stamina: p.Stamina, MaxStamina: p.MaxStamina,
			MP: p.MP, MaxMP: p.MaxMP,
			Attack: p.Attack, Defense: p.Defense,
			EXP: p.EXP,
		}
		// If in combat, remove from fight
		if p.FightID != 0 {
			if fight, ok := gl.fights[p.FightID]; ok {
				fight.RemovePlayer(p.ID)
			}
			p.FightID = 0
		}
		p.Dead = false
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

	// Update animations and interactions for all players
	gl.mu.RLock()
	for _, p := range gl.players {
		updatePlayerAnimation(p)
		p.ActiveInteraction = gl.computeInteraction(p)
	}
	gl.mu.RUnlock()

	// Tick combat state machines
	gl.mu.RLock()
	gl.tickCombat()
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
		// Attach combat state if player is in a fight
		if p.FightID != 0 {
			if fight, ok := gl.fights[p.FightID]; ok {
				state.Combat = fight.Snapshot(id, gl.players)
			}
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
			p.AnimFrame = (p.AnimFrame + 1) % 6
			p.AnimTick = 0
		}
	} else {
		// Idle animation
		if p.AnimTick >= IdleFrameInterval {
			p.AnimFrame = (p.AnimFrame + 1) % 6
			p.AnimTick = 0
		}
	}
}

// computeInteraction checks if the player is facing an interaction tile.
func (gl *GameLoop) computeInteraction(p *Player) *ActiveInteraction {
	fx, fy := p.X, p.Y
	switch p.Dir {
	case DirUp:
		fy--
	case DirDown:
		fy++
	case DirLeft:
		fx--
	case DirRight:
		fx++
	}
	inter := gl.world.InteractionAt(p.MapName, fx, fy)
	if inter == nil {
		return nil
	}
	return &ActiveInteraction{WorldX: inter.X, WorldY: inter.Y, Text: inter.Text}
}

func (gl *GameLoop) processInput(ev InputEvent) {
	gl.mu.RLock()
	player, ok := gl.players[ev.PlayerID]
	gl.mu.RUnlock()
	if !ok {
		return
	}

	// In combat: route to combat input handler
	if player.FightID != 0 {
		gl.processCombatInput(player, ev.Action)
		return
	}

	// Toggle debug view
	if ev.Action == ActionDebug {
		player.DebugView = !player.DebugView
		return
	}

	// Toggle tile debug overlay
	if ev.Action == ActionDebugTileOverlay {
		player.DebugTileOverlay = !player.DebugTileOverlay
		return
	}

	// Debug: force-start combat encounter from anywhere
	if ev.Action == ActionDebugCombat {
		if player.FightID == 0 && !player.Dead {
			gl.startEncounter(player)
		}
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

	// Ignore page/combat actions outside debug/combat
	switch ev.Action {
	case ActionDebugPage1, ActionDebugPage2, ActionDebugPage3, ActionConfirm, ActionDefend:
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
		} else {
			// Check for encounter on tall_grass
			gl.checkEncounter(player)
		}
	}
}

// checkEncounter triggers a random combat encounter on tall_grass tiles.
func (gl *GameLoop) checkEncounter(player *Player) {
	m := gl.world.GetMap(player.MapName)
	if m == nil {
		return
	}
	tile := m.TileAt(player.X, player.Y)
	if tile.Name != "tall_grass" {
		return
	}
	if rand.Intn(100) >= EncounterChance {
		return
	}
	gl.startEncounter(player)
}

// startEncounter creates a fight and pulls all same-map non-combat players in.
func (gl *GameLoop) startEncounter(trigger *Player) {
	gl.nextFightID++
	fightID := gl.nextFightID

	// Gather all non-combat, non-dead players on the same map
	playerIDs := []string{trigger.ID}
	trigger.CombatTransition = CombatTransitionLen
	trigger.FightID = fightID
	trigger.CombatAction = 0
	trigger.CombatTarget = 0

	for _, p := range gl.players {
		if p.ID == trigger.ID {
			continue
		}
		if p.MapName == trigger.MapName && p.FightID == 0 && !p.Dead {
			p.FightID = fightID
			p.CombatTransition = CombatCoopTransLen
			p.CombatAction = 0
			p.CombatTarget = 0
			playerIDs = append(playerIDs, p.ID)
		}
	}

	fight := NewFight(fightID, trigger.MapName, playerIDs)
	gl.fights[fightID] = fight
}

// processCombatInput handles input for a player in combat.
func (gl *GameLoop) processCombatInput(player *Player, action Action) {
	fight, ok := gl.fights[player.FightID]
	if !ok {
		return
	}

	// Can't act during transition, enemy turn, or result screens
	if fight.Phase != PhasePlayerTurn {
		return
	}
	// Can't act if still in transition
	if player.CombatTransition > 0 {
		return
	}
	// Dead players spectate
	if player.Dead {
		return
	}
	// Only the current turn player can act
	if fight.CurrentTurnPlayerID() != player.ID {
		return
	}

	livingEnemies := fight.LivingEnemies()
	if len(livingEnemies) == 0 {
		return
	}

	switch action {
	case ActionDebugPage1: // key '1' = Melee
		player.CombatAction = 1
	case ActionDebugPage2: // key '2' = Ranged
		player.CombatAction = 2
	case ActionDebugPage3: // key '3' = Magic
		player.CombatAction = 3
	case ActionDefend: // key '4' = Defend
		msg := ResolveDefend(player)
		fight.AddLog(msg)
		gl.advanceCombatTurn(fight)
		return
	case ActionLeft:
		// Cycle target left
		if player.CombatTarget > 0 {
			player.CombatTarget--
		} else {
			player.CombatTarget = len(livingEnemies) - 1
		}
	case ActionRight:
		// Cycle target right
		player.CombatTarget = (player.CombatTarget + 1) % len(livingEnemies)
	case ActionConfirm:
		// Confirm selected action on selected target
		if player.CombatAction == 0 {
			return // no action selected
		}
		// Clamp target to living enemies
		if player.CombatTarget >= len(livingEnemies) {
			player.CombatTarget = 0
		}
		target := livingEnemies[player.CombatTarget]

		var ok bool
		var msg string
		switch player.CombatAction {
		case 1: // Melee
			_, msg, ok = ResolveMelee(player, target)
		case 2: // Ranged
			_, msg, ok = ResolveRanged(player, target)
		case 3: // Magic
			_, msg, ok = ResolveMagic(player, target)
		}
		if !ok {
			return // not enough resources
		}
		fight.AddLog(msg)
		player.CombatAction = 0
		gl.advanceCombatTurn(fight)
	}
}

// advanceCombatTurn moves to the next player or enemy phase.
func (gl *GameLoop) advanceCombatTurn(fight *Fight) {
	// Check if all enemies are dead
	if fight.AllEnemiesDead() {
		fight.Phase = PhaseVictory
		fight.ResultTimer = CombatResultDelay
		fight.AddLog("Victory! All enemies defeated!")
		return
	}

	// Try next player
	if fight.NextPlayerTurn(gl.players) {
		return
	}

	// All players have acted — enemy phase
	fight.StartEnemyPhase()
}

// tickCombat advances all active fights each tick.
func (gl *GameLoop) tickCombat() {
	// Decrement combat transitions for all players
	for _, p := range gl.players {
		if p.CombatTransition > 0 {
			p.CombatTransition--
		}
	}

	var finishedFights []int

	for fid, fight := range gl.fights {
		// Clean up fights with no players remaining (all disconnected)
		if len(fight.PlayerIDs) == 0 {
			finishedFights = append(finishedFights, fid)
			continue
		}

		switch fight.Phase {
		case PhaseTransition:
			// Wait for all players' transitions to end
			allReady := true
			for _, pid := range fight.PlayerIDs {
				if p, ok := gl.players[pid]; ok && p.CombatTransition > 0 {
					allReady = false
					break
				}
			}
			if allReady {
				fight.StartPlayerPhase(gl.players)
			}

		case PhasePlayerTurn:
			// Turn timer countdown
			fight.TurnTimer--
			if fight.TurnTimer <= 0 {
				// Auto-defend on timeout
				pid := fight.CurrentTurnPlayerID()
				if p, ok := gl.players[pid]; ok && !p.Dead {
					msg := ResolveDefend(p)
					fight.AddLog(msg + " (timeout)")
				}
				gl.advanceCombatTurn(fight)
			}

		case PhaseEnemyTurn:
			// Start the first enemy's action
			gl.tickEnemyActions(fight)

		case PhaseEnemyActing:
			fight.EnemyTimer--
			if fight.EnemyTimer <= 0 {
				fight.Phase = PhaseEnemyTurn
				fight.EnemyIndex++
				gl.tickEnemyActions(fight)
			}

		case PhaseVictory:
			fight.ResultTimer--
			if fight.ResultTimer <= 0 {
				gl.resolveFightVictory(fight)
				finishedFights = append(finishedFights, fid)
			}

		case PhaseDefeat:
			fight.ResultTimer--
			if fight.ResultTimer <= 0 {
				gl.resolveFightDefeat(fight)
				finishedFights = append(finishedFights, fid)
			}
		}
	}

	for _, fid := range finishedFights {
		delete(gl.fights, fid)
	}
}

// tickEnemyActions processes the current enemy's attack.
func (gl *GameLoop) tickEnemyActions(fight *Fight) {
	// Find next living enemy from current index
	for fight.EnemyIndex < len(fight.Enemies) {
		enemy := fight.Enemies[fight.EnemyIndex]
		if !enemy.Alive() {
			fight.EnemyIndex++
			continue
		}

		// Pick random living player target
		living := fight.LivingPlayers(gl.players)
		if len(living) == 0 {
			// All players dead
			fight.Phase = PhaseDefeat
			fight.ResultTimer = CombatResultDelay
			fight.AddLog("Defeat! All players have fallen!")
			return
		}

		targetID := living[rand.Intn(len(living))]
		target := gl.players[targetID]
		_, msg := ResolveEnemyAttack(enemy, target)
		fight.AddLog(msg)

		// Check if all players are now dead
		if fight.LivingPlayerCount(gl.players) == 0 {
			fight.Phase = PhaseDefeat
			fight.ResultTimer = CombatResultDelay
			fight.AddLog("Defeat! All players have fallen!")
			return
		}

		// Delay before next enemy
		fight.Phase = PhaseEnemyActing
		fight.EnemyTimer = CombatEnemyActDelay
		return
	}

	// All enemies have acted — new round
	fight.Round++
	fight.StartPlayerPhase(gl.players)
}

// resolveFightVictory awards EXP and returns players to the overworld.
func (gl *GameLoop) resolveFightVictory(fight *Fight) {
	totalEXP := fight.TotalEXP()
	for _, pid := range fight.PlayerIDs {
		if p, ok := gl.players[pid]; ok {
			if !p.Dead {
				p.EXP += totalEXP
			}
			p.FightID = 0
			p.Dead = false
			p.Defending = false
			p.CombatAction = 0
			p.CombatTarget = 0
			p.CombatTransition = 0
		}
	}
}

// resolveFightDefeat respawns all players at town with full stats.
func (gl *GameLoop) resolveFightDefeat(fight *Fight) {
	mapName, spawnX, spawnY := gl.world.SpawnPoint()
	for _, pid := range fight.PlayerIDs {
		if p, ok := gl.players[pid]; ok {
			p.FightID = 0
			p.Dead = false
			p.Defending = false
			p.CombatAction = 0
			p.CombatTarget = 0
			p.CombatTransition = 0
			p.HP = p.MaxHP
			p.Stamina = p.MaxStamina
			p.MP = p.MaxMP
			p.MapName = mapName
			p.X = spawnX
			p.Y = spawnY
		}
	}
}
