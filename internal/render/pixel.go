package render

const (
	// PixelTileW is the width of a pixel sprite in pixels.
	PixelTileW = 16
	// PixelTileH is the height of a pixel sprite in pixels.
	PixelTileH = 16

	// CharTileW is the width of a pixel tile in terminal columns (1 pixel = 1 column).
	CharTileW = 16
	// CharTileH is the height of a pixel tile in terminal rows (2 pixels per row via half-block).
	CharTileH = 8
)

// Pixel represents a single pixel with RGB color and transparency.
type Pixel struct {
	R, G, B     uint8
	Transparent bool
}

// PixelSprite is a PixelTileH x PixelTileW grid of pixels.
type PixelSprite [PixelTileH][PixelTileW]Pixel

// PlayerSpriteH is the height of a player sprite in pixels (16 body + 4 hair above).
const PlayerSpriteH = 20

// PlayerSprite is a 20x16 pixel grid for player characters.
// 4 pixels taller than a tile sprite to accommodate hair above the tile boundary.
type PlayerSprite [PlayerSpriteH][PixelTileW]Pixel

// PixelOverlay is a pixel sprite rendered at an offset from its owning tile.
type PixelOverlay struct {
	Sprite PixelSprite
	DY     int // tile units upward (1 = one tile above base)
	DX     int // tile units horizontal (-1 = one tile left, +1 = one tile right)
}

// PixelTileSprites holds the base pixel sprite and optional overlay layers.
type PixelTileSprites struct {
	Base     PixelSprite
	Overlays []PixelOverlay
}

// TransparentPixel returns a transparent pixel.
func TransparentPixel() Pixel {
	return Pixel{Transparent: true}
}

// P is a shorthand to create an opaque pixel.
func P(r, g, b uint8) Pixel {
	return Pixel{R: r, G: g, B: b}
}

// FillPixelSprite creates a pixel sprite filled with a single color.
func FillPixelSprite(r, g, b uint8) PixelSprite {
	var s PixelSprite
	p := P(r, g, b)
	for y := 0; y < PixelTileH; y++ {
		for x := 0; x < PixelTileW; x++ {
			s[y][x] = p
		}
	}
	return s
}

// TransparentPixelSprite creates a fully transparent pixel sprite.
func TransparentPixelSprite() PixelSprite {
	var s PixelSprite
	for y := 0; y < PixelTileH; y++ {
		for x := 0; x < PixelTileW; x++ {
			s[y][x] = TransparentPixel()
		}
	}
	return s
}
