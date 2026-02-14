package render

// PlayerSprite returns the 10x5 sprite for a player character.
// Uses 2-col-wide "pixels" for a clean block-art look.
func PlayerSprite(dir, anim, frame, color int, isSelf bool, name string) Sprite {
	colorIdx := color % len(PlayerBGColors)
	bgR, bgG, bgB := PlayerBGColors[colorIdx][0], PlayerBGColors[colorIdx][1], PlayerBGColors[colorIdx][2]

	switch dir {
	case 1:
		return playerUp(bgR, bgG, bgB)
	case 2:
		return playerLeft(bgR, bgG, bgB)
	case 3:
		return playerRight(bgR, bgG, bgB)
	default:
		return playerDown(bgR, bgG, bgB)
	}
}

var t = TransparentCell()

func clearSprite() Sprite {
	var s Sprite
	for y := 0; y < TileHeight; y++ {
		for x := 0; x < TileWidth; x++ {
			s[y][x] = t
		}
	}
	return s
}

// px fills a 2-column "pixel" at pixel position p (0-4) in the given row.
func px(s *Sprite, row, p int, r, g, b uint8) {
	col := p * 2
	s[row][col] = SpriteCell{Cell: Cell{Ch: ' ', BgR: r, BgG: g, BgB: b}}
	s[row][col+1] = SpriteCell{Cell: Cell{Ch: ' ', BgR: r, BgG: g, BgB: b}}
}

// Fixed palette
const (
	hairR, hairG, hairB = 100, 60, 25  // warm chestnut brown
	skinR, skinG, skinB = 237, 195, 155 // warm golden peach
	eyeR, eyeG, eyeB   = 30, 20, 15    // deep brown (softer than black)
	shoeR, shoeG, shoeB = 62, 42, 28   // dark leather brown
)

// pant darkens the player shirt color for pants contrast.
func pant(r, g, b uint8) (uint8, uint8, uint8) {
	return r * 2 / 3, g * 2 / 3, b * 2 / 3
}

// --- DOWN (front) ---
//  _  BR BR BR _
//  _  BK SK BK _
//  BL BL BL BL BL
//  _  BL BL BL _
//  _  SH _  SH _
func playerDown(bR, bG, bB uint8) Sprite {
	s := clearSprite()
	pR, pG, pB := pant(bR, bG, bB)
	// Row 0: hair
	px(&s, 0, 1, hairR, hairG, hairB)
	px(&s, 0, 2, hairR, hairG, hairB)
	px(&s, 0, 3, hairR, hairG, hairB)
	// Row 1: face — skin with eye dots
	px(&s, 1, 1, skinR, skinG, skinB)
	px(&s, 1, 2, skinR, skinG, skinB)
	px(&s, 1, 3, skinR, skinG, skinB)
	s[1][3] = SC('o', eyeR, eyeG, eyeB, skinR, skinG, skinB)
	s[1][6] = SC('o', eyeR, eyeG, eyeB, skinR, skinG, skinB)
	// Row 2: shirt (full width)
	px(&s, 2, 0, bR, bG, bB)
	px(&s, 2, 1, bR, bG, bB)
	px(&s, 2, 2, bR, bG, bB)
	px(&s, 2, 3, bR, bG, bB)
	px(&s, 2, 4, bR, bG, bB)
	// Row 3: pants (darkened shirt color)
	px(&s, 3, 1, pR, pG, pB)
	px(&s, 3, 2, pR, pG, pB)
	px(&s, 3, 3, pR, pG, pB)
	// Row 4: shoes
	px(&s, 4, 1, shoeR, shoeG, shoeB)
	px(&s, 4, 3, shoeR, shoeG, shoeB)
	return s
}

// --- UP (back) ---
//  _  BR BR BR _
//  _  BR BR BR _
//  BL BL BL BL BL
//  _  BL BL BL _
//  _  SH _  SH _
func playerUp(bR, bG, bB uint8) Sprite {
	s := clearSprite()
	pR, pG, pB := pant(bR, bG, bB)
	// Row 0: hair
	px(&s, 0, 1, hairR, hairG, hairB)
	px(&s, 0, 2, hairR, hairG, hairB)
	px(&s, 0, 3, hairR, hairG, hairB)
	// Row 1: back of head (all hair)
	px(&s, 1, 1, hairR, hairG, hairB)
	px(&s, 1, 2, hairR, hairG, hairB)
	px(&s, 1, 3, hairR, hairG, hairB)
	// Row 2: shirt (full width)
	px(&s, 2, 0, bR, bG, bB)
	px(&s, 2, 1, bR, bG, bB)
	px(&s, 2, 2, bR, bG, bB)
	px(&s, 2, 3, bR, bG, bB)
	px(&s, 2, 4, bR, bG, bB)
	// Row 3: pants
	px(&s, 3, 1, pR, pG, pB)
	px(&s, 3, 2, pR, pG, pB)
	px(&s, 3, 3, pR, pG, pB)
	// Row 4: shoes
	px(&s, 4, 1, shoeR, shoeG, shoeB)
	px(&s, 4, 3, shoeR, shoeG, shoeB)
	return s
}

// --- RIGHT ---
//  _  _  BR BR _
//  _  _  SK BK _
//  _  BL BL BL BL
//  _  _  BL BL _
//  _  _  SH _  SH
func playerRight(bR, bG, bB uint8) Sprite {
	s := clearSprite()
	pR, pG, pB := pant(bR, bG, bB)
	// Row 0: hair
	px(&s, 0, 2, hairR, hairG, hairB)
	px(&s, 0, 3, hairR, hairG, hairB)
	// Row 1: face — hair, skin with eye + ear
	px(&s, 1, 2, hairR, hairG, hairB)
	px(&s, 1, 3, skinR, skinG, skinB)
	s[1][6] = SC('(', skinR-40, skinG-40, skinB-30, skinR, skinG, skinB)
	s[1][7] = SC('o', eyeR, eyeG, eyeB, skinR, skinG, skinB)
	// Row 2: shirt
	px(&s, 2, 1, bR, bG, bB)
	px(&s, 2, 2, bR, bG, bB)
	px(&s, 2, 3, bR, bG, bB)
	px(&s, 2, 4, bR, bG, bB)
	// Row 3: pants
	px(&s, 3, 2, pR, pG, pB)
	px(&s, 3, 3, pR, pG, pB)
	// Row 4: shoes
	px(&s, 4, 2, shoeR, shoeG, shoeB)
	px(&s, 4, 4, shoeR, shoeG, shoeB)
	return s
}

// --- LEFT ---
//  _  BR BR _  _
//  _  BK SK _  _
//  BL BL BL BL _
//  _  BL BL _  _
//  SH _  SH _  _
func playerLeft(bR, bG, bB uint8) Sprite {
	s := clearSprite()
	pR, pG, pB := pant(bR, bG, bB)
	// Row 0: hair
	px(&s, 0, 1, hairR, hairG, hairB)
	px(&s, 0, 2, hairR, hairG, hairB)
	// Row 1: face — skin with ear + eye, hair
	px(&s, 1, 1, skinR, skinG, skinB)
	px(&s, 1, 2, hairR, hairG, hairB)
	s[1][2] = SC('o', eyeR, eyeG, eyeB, skinR, skinG, skinB)
	s[1][3] = SC(')', skinR-40, skinG-40, skinB-30, skinR, skinG, skinB)
	// Row 2: shirt
	px(&s, 2, 0, bR, bG, bB)
	px(&s, 2, 1, bR, bG, bB)
	px(&s, 2, 2, bR, bG, bB)
	px(&s, 2, 3, bR, bG, bB)
	// Row 3: pants
	px(&s, 3, 1, pR, pG, pB)
	px(&s, 3, 2, pR, pG, pB)
	// Row 4: shoes
	px(&s, 4, 0, shoeR, shoeG, shoeB)
	px(&s, 4, 2, shoeR, shoeG, shoeB)
	return s
}
