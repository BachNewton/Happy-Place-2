package game

const TickRate = 20 // ticks per second

// SecsToTicks converts a duration in seconds to game ticks.
func SecsToTicks(s float64) int {
	t := int(s * TickRate)
	if t < 1 {
		t = 1
	}
	return t
}

// Timing constants — all expressed in seconds, converted to ticks at init.
var (
	MoveRepeatDelay   = SecsToTicks(0.15) // min ticks between moves when holding a key
	WalkAnimDuration  = SecsToTicks(0.4)  // how long walk animation plays after a move
	WalkFrameInterval = SecsToTicks(0.2)  // ticks between walk animation frames
	IdleFrameInterval = SecsToTicks(1.0)  // ticks between idle animation frames
	WaterAnimInterval = SecsToTicks(0.4)  // ticks between water animation frames
	GrassAnimInterval = SecsToTicks(2.0)  // ticks between grass wind sway frames

	// Combat timing
	CombatTurnTimeout   = SecsToTicks(15.0) // auto-defend after this many ticks
	CombatEnemyActDelay = SecsToTicks(1.0)  // pause between enemy actions
	CombatTransitionLen = SecsToTicks(1.0)  // screen flash duration for trigger player
	CombatCoopTransLen  = SecsToTicks(0.5)  // shorter transition for pulled-in players
	CombatResultDelay   = SecsToTicks(3.0)  // victory/defeat screen duration
)

// EncounterChance is the percent chance per tall_grass step.
const EncounterChance = 15
