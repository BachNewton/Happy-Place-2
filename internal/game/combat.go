package game

// CombatPhase tracks the current phase of a fight.
type CombatPhase int

const (
	PhaseTransition CombatPhase = iota // screen flash before combat starts
	PhasePlayerTurn                    // waiting for a player to act
	PhaseEnemyTurn                     // enemies acting sequentially
	PhaseEnemyActing                   // delay between enemy actions
	PhaseVictory                       // all enemies dead
	PhaseDefeat                        // all players dead
)

// CombatState is the snapshot sent to the renderer for a player in combat.
type CombatState struct {
	Phase        CombatPhase
	Round        int
	Enemies      []EnemySnapshot
	Players      []CombatPlayerSnapshot
	CurrentTurn  string // player ID whose turn it is (empty during enemy/transition)
	TurnTimer    int    // ticks remaining for current turn
	Log          []string
	ViewerID     string // who this snapshot is for
	Transitioning bool  // true if the viewer is still in transition
	ViewerAction int    // selected action (1=Melee,2=Ranged,3=Magic, 0=none)
	ViewerTarget int    // selected enemy target index
}

// EnemySnapshot is a read-only view of an enemy for rendering.
type EnemySnapshot struct {
	Label string
	HP    int
	MaxHP int
	ID    int
	Alive bool
}

// CombatPlayerSnapshot is a read-only view of a player in combat.
type CombatPlayerSnapshot struct {
	ID      string
	Name    string
	HP      int
	MaxHP   int
	Alive   bool
	Color   int
	IsViewer bool
}

// Fight manages the state of a single combat encounter.
type Fight struct {
	ID         int
	MapName    string
	Round      int
	Phase      CombatPhase
	Enemies    []*EnemyInstance
	PlayerIDs  []string // ordered: trigger player first
	TurnIndex  int      // index into PlayerIDs for current turn
	TurnTimer  int      // ticks until auto-defend
	EnemyIndex int      // which enemy is currently acting
	EnemyTimer int      // ticks until next enemy acts
	ResultTimer int     // ticks remaining on victory/defeat screen
	Log        []string // battle log messages (most recent last)
}

const maxLogLines = 6

// NewFight creates a fight with enemies matching the player count.
func NewFight(id int, mapName string, playerIDs []string) *Fight {
	enemies := spawnEnemies(EnemyRat, len(playerIDs))
	return &Fight{
		ID:        id,
		MapName:   mapName,
		Round:     1,
		Phase:     PhaseTransition,
		Enemies:   enemies,
		PlayerIDs: playerIDs,
	}
}

// AddLog appends a message to the battle log, keeping it trimmed.
func (f *Fight) AddLog(msg string) {
	f.Log = append(f.Log, msg)
	if len(f.Log) > maxLogLines {
		f.Log = f.Log[len(f.Log)-maxLogLines:]
	}
}

// CurrentTurnPlayerID returns the player ID whose turn it is, or "" if not a player turn.
func (f *Fight) CurrentTurnPlayerID() string {
	if f.Phase != PhasePlayerTurn {
		return ""
	}
	if f.TurnIndex < 0 || f.TurnIndex >= len(f.PlayerIDs) {
		return ""
	}
	return f.PlayerIDs[f.TurnIndex]
}

// AllEnemiesDead returns true if every enemy has been defeated.
func (f *Fight) AllEnemiesDead() bool {
	for _, e := range f.Enemies {
		if e.Alive() {
			return false
		}
	}
	return true
}

// AllPlayersDead checks if all players in the fight are dead.
func (f *Fight) AllPlayersDead(players map[string]*Player) bool {
	for _, pid := range f.PlayerIDs {
		if p, ok := players[pid]; ok && !p.Dead {
			return false
		}
	}
	return true
}

// LivingPlayerCount returns how many players are still alive.
func (f *Fight) LivingPlayerCount(players map[string]*Player) int {
	count := 0
	for _, pid := range f.PlayerIDs {
		if p, ok := players[pid]; ok && !p.Dead {
			count++
		}
	}
	return count
}

// LivingPlayers returns the IDs of living players.
func (f *Fight) LivingPlayers(players map[string]*Player) []string {
	var result []string
	for _, pid := range f.PlayerIDs {
		if p, ok := players[pid]; ok && !p.Dead {
			result = append(result, p.ID)
		}
	}
	return result
}

// LivingEnemies returns the living enemy instances.
func (f *Fight) LivingEnemies() []*EnemyInstance {
	var result []*EnemyInstance
	for _, e := range f.Enemies {
		if e.Alive() {
			result = append(result, e)
		}
	}
	return result
}

// NextPlayerTurn advances to the next living player's turn within the current round.
// Only searches forward from TurnIndex+1 to the end of the list (no wrapping).
// Returns false when all remaining players in this round have acted.
func (f *Fight) NextPlayerTurn(players map[string]*Player) bool {
	for idx := f.TurnIndex + 1; idx < len(f.PlayerIDs); idx++ {
		pid := f.PlayerIDs[idx]
		if p, ok := players[pid]; ok && !p.Dead {
			f.TurnIndex = idx
			f.TurnTimer = CombatTurnTimeout
			p.CombatAction = 0
			p.CombatTarget = 0
			return true
		}
	}
	return false
}

// StartPlayerPhase begins the player turn phase from the first living player.
func (f *Fight) StartPlayerPhase(players map[string]*Player) {
	f.Phase = PhasePlayerTurn
	f.TurnIndex = -1
	// Clear defending flag for all players at start of round
	for _, pid := range f.PlayerIDs {
		if p, ok := players[pid]; ok {
			p.Defending = false
		}
	}
	if !f.NextPlayerTurn(players) {
		// No living players, go to enemy turn
		f.StartEnemyPhase()
	}
}

// StartEnemyPhase begins the enemy action phase.
func (f *Fight) StartEnemyPhase() {
	f.Phase = PhaseEnemyTurn
	f.EnemyIndex = 0
	f.EnemyTimer = CombatEnemyActDelay
}

// RemovePlayer removes a player from the fight (on disconnect).
func (f *Fight) RemovePlayer(playerID string) {
	for i, pid := range f.PlayerIDs {
		if pid == playerID {
			f.PlayerIDs = append(f.PlayerIDs[:i], f.PlayerIDs[i+1:]...)
			break
		}
	}
}

// Snapshot builds a CombatState for the given viewer.
func (f *Fight) Snapshot(viewerID string, players map[string]*Player) *CombatState {
	enemies := make([]EnemySnapshot, len(f.Enemies))
	for i, e := range f.Enemies {
		enemies[i] = EnemySnapshot{
			Label: e.Label,
			HP:    e.HP,
			MaxHP: e.Def.MaxHP,
			ID:    e.ID,
			Alive: e.Alive(),
		}
	}

	combatPlayers := make([]CombatPlayerSnapshot, 0, len(f.PlayerIDs))
	for _, pid := range f.PlayerIDs {
		p, ok := players[pid]
		if !ok {
			continue
		}
		combatPlayers = append(combatPlayers, CombatPlayerSnapshot{
			ID:       p.ID,
			Name:     p.Name,
			HP:       p.HP,
			MaxHP:    p.MaxHP,
			Alive:    !p.Dead,
			Color:    p.Color,
			IsViewer: p.ID == viewerID,
		})
	}

	transitioning := false
	if p, ok := players[viewerID]; ok {
		transitioning = p.CombatTransition > 0
	}

	logCopy := make([]string, len(f.Log))
	copy(logCopy, f.Log)

	var viewerAction, viewerTarget int
	if p, ok := players[viewerID]; ok {
		viewerAction = p.CombatAction
		viewerTarget = p.CombatTarget
	}

	return &CombatState{
		Phase:         f.Phase,
		Round:         f.Round,
		Enemies:       enemies,
		Players:       combatPlayers,
		CurrentTurn:   f.CurrentTurnPlayerID(),
		TurnTimer:     f.TurnTimer,
		Log:           logCopy,
		ViewerID:      viewerID,
		Transitioning: transitioning,
		ViewerAction:  viewerAction,
		ViewerTarget:  viewerTarget,
	}
}

// TotalEXP returns the total EXP from all enemies in the fight.
func (f *Fight) TotalEXP() int {
	total := 0
	for _, e := range f.Enemies {
		total += e.Def.EXP
	}
	return total
}
