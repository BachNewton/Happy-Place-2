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
)
