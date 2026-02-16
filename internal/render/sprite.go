package render

const (
	// TileWidth is how many screen columns each world tile occupies.
	TileWidth = 10

	// TileHeight is how many screen rows each world tile occupies.
	TileHeight = 5
)

// SpriteCell is a single cell within a sprite, with optional transparency.
type SpriteCell struct {
	Cell        Cell
	Transparent bool // true = show tile underneath (for player overlay)
}

// Sprite is a TileHeight x TileWidth grid of sprite cells.
type Sprite [TileHeight][TileWidth]SpriteCell

// AnimatedSprite holds multiple frames for animation.
type AnimatedSprite struct {
	Frames   []Sprite
	TickRate int // game ticks per frame advance
}

// Overlay is a sprite rendered at a vertical offset above its owning tile.
type Overlay struct {
	Sprite Sprite
	DY     int // tile units upward (1 = one tile above base)
}

// TileSprites holds the base sprite and optional overlay layers.
type TileSprites struct {
	Base     Sprite
	Overlays []Overlay
}

// TransparentCell returns a SpriteCell that lets the tile underneath show through.
func TransparentCell() SpriteCell {
	return SpriteCell{Transparent: true}
}

// SC is a shorthand to create an opaque SpriteCell.
func SC(ch rune, fgR, fgG, fgB, bgR, bgG, bgB uint8) SpriteCell {
	return SpriteCell{
		Cell: Cell{Ch: ch, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB},
	}
}

// SCBold creates an opaque bold SpriteCell.
func SCBold(ch rune, fgR, fgG, fgB, bgR, bgG, bgB uint8) SpriteCell {
	return SpriteCell{
		Cell: Cell{Ch: ch, FgR: fgR, FgG: fgG, FgB: fgB, BgR: bgR, BgG: bgG, BgB: bgB, Bold: true},
	}
}

// FillSprite creates a sprite filled with a single character and color.
func FillSprite(ch rune, fgR, fgG, fgB, bgR, bgG, bgB uint8) Sprite {
	var s Sprite
	c := SC(ch, fgR, fgG, fgB, bgR, bgG, bgB)
	for y := 0; y < TileHeight; y++ {
		for x := 0; x < TileWidth; x++ {
			s[y][x] = c
		}
	}
	return s
}
