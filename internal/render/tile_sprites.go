package render

import "happy-place-2/internal/maps"

// tileFunc generates a sprite for a tile at world position (wx,wy) at the given tick.
type tileFunc func(wx, wy int, tick uint64) Sprite

// tileEntry holds a tile's sprite generator and how many variants it has.
type tileEntry struct {
	fn       tileFunc
	variants int // number of distinct variants (1 = no variation)
}

// tileNames is the ordered list of all known tile types (used by debug view).
var tileNames = []string{"grass", "wall", "water", "tree", "path", "door", "floor", "fence"}

// tileRegistry maps tile names to their sprite generators and variant counts.
var tileRegistry = map[string]tileEntry{
	"grass": {func(wx, wy int, tick uint64) Sprite { return grassSprite(uint(wx*7+wy*13)%4, tick) }, 4},
	"wall":  {func(wx, wy int, _ uint64) Sprite { return wallSprite(wx, wy) }, 1},
	"water": {func(wx, wy int, tick uint64) Sprite { return waterSprite(wx, wy, tick) }, 1},
	"tree":  {func(wx, wy int, _ uint64) Sprite { return treeSprite(uint(wx*7+wy*13) % 4) }, 4},
	"path":  {func(wx, wy int, _ uint64) Sprite { return pathSprite(uint(wx*7+wy*13) % 4) }, 4},
	"door":  {func(_, _ int, _ uint64) Sprite { return doorSprite() }, 1},
	"floor": {func(wx, wy int, _ uint64) Sprite { return floorSprite(uint(wx*7+wy*13) % 4) }, 4},
	"fence": {func(wx, wy int, _ uint64) Sprite { return fenceSprite(uint(wx*7+wy*13) % 4) }, 4},
}

// TileNames returns the ordered list of all known tile type names.
func TileNames() []string {
	return tileNames
}

// TileVariants returns the number of visual variants for the given tile name.
func TileVariants(name string) int {
	if e, ok := tileRegistry[name]; ok {
		return e.variants
	}
	return 1
}

// TileSprite returns the sprite for a tile at world position (wx,wy) at the given tick.
func TileSprite(tile maps.TileDef, wx, wy int, tick uint64) Sprite {
	if e, ok := tileRegistry[tile.Name]; ok {
		return e.fn(wx, wy, tick)
	}
	return fallbackSprite(tile)
}

// --- Grass ---

func grassSprite(v uint, tick uint64) Sprite {
	bgR, bgG, bgB := uint8(28), uint8(65), uint8(28)
	bgG += uint8(v * 3)

	s := FillSprite(' ', 0, 0, 0, bgR, bgG, bgB)

	type blade struct {
		ch         rune
		fr, fg, fb uint8
	}
	blades := [4]blade{
		{',', 60, 135, 50},
		{'.', 50, 115, 42},
		{'\'', 70, 145, 55},
		{'`', 55, 125, 48},
	}

	type pos struct{ x, y int }
	patterns := [4][]pos{
		{{1, 1}, {5, 2}, {8, 4}, {3, 3}, {7, 0}},
		{{2, 0}, {6, 3}, {9, 1}, {1, 4}, {4, 2}},
		{{0, 1}, {4, 3}, {8, 4}, {2, 2}, {6, 0}},
		{{3, 0}, {7, 2}, {1, 4}, {5, 1}, {9, 3}},
	}

	frame := int(tick/uint64(max(8, 1))) % 2

	for i, p := range patterns[v] {
		b := blades[i%len(blades)]
		x := (p.x + frame) % TileWidth
		s[p.y][x] = SC(b.ch, b.fr, b.fg, b.fb, bgR, bgG, bgB)
	}

	return s
}

// --- Wall ---

func wallSprite(wx, wy int) Sprite {
	stoneR, stoneG, stoneB := uint8(100), uint8(100), uint8(110)
	mortarR, mortarG, mortarB := uint8(60), uint8(60), uint8(70)

	s := FillSprite('▓', stoneR, stoneG, stoneB, mortarR, mortarG, mortarB)

	// Horizontal mortar at rows 0 and 3
	for _, row := range []int{0, 3} {
		for x := 0; x < TileWidth; x++ {
			s[row][x] = SC('░', mortarR+20, mortarG+20, mortarB+20, mortarR, mortarG, mortarB)
		}
	}

	// Vertical mortar — staggered
	for y := 1; y < TileHeight; y++ {
		if y == 3 {
			continue
		}
		vOff := 0
		if y > 3 {
			vOff = 5
		}
		mortarX := (vOff + (wx * 7)) % TileWidth
		s[y][mortarX] = SC('░', mortarR+20, mortarG+20, mortarB+20, mortarR, mortarG, mortarB)
	}

	v := uint(wx*3+wy*11) % 4
	if v == 0 {
		s[2][3] = SC('▒', stoneR-10, stoneG-10, stoneB-5, mortarR, mortarG, mortarB)
	} else if v == 1 {
		s[1][7] = SC('▒', stoneR-15, stoneG-15, stoneB-10, mortarR, mortarG, mortarB)
	}

	return s
}

// --- Water ---

func waterSprite(wx, wy int, tick uint64) Sprite {
	bgR, bgG, bgB := uint8(15), uint8(38), uint8(95)
	fgR, fgG, fgB := uint8(70), uint8(130), uint8(210)

	frame := int(tick/uint64(max(8, 1))) % 4

	s := FillSprite(' ', fgR, fgG, fgB, bgR, bgG, bgB)

	waveChars := []rune{'~', '~', '≈', ' ', '~', ' ', '≈', '~'}

	for y := 0; y < TileHeight; y++ {
		rowPhase := (y*3 + wx*5 + wy*7) % len(waveChars)
		for x := 0; x < TileWidth; x++ {
			charIdx := (x + rowPhase + frame*3) % len(waveChars)
			ch := waveChars[charIdx]

			shimmer := uint8(((x + y*2 + int(tick/6)) % 3) * 12)
			cellFgB := fgB + shimmer
			if cellFgB < fgB {
				cellFgB = 255
			}

			rowBgB := bgB + uint8(y*4)
			if rowBgB < bgB {
				rowBgB = 255
			}

			s[y][x] = SC(ch, fgR, fgG, cellFgB, bgR, bgG, rowBgB)
		}
	}

	crestY := (frame + wx) % TileHeight
	for x := 0; x < TileWidth; x += 4 {
		cx := (x + frame*2) % TileWidth
		s[crestY][cx] = SCBold('≈', fgR+40, fgG+40, min8(fgB+60, 255), bgR, bgG+5, bgB+10)
	}

	return s
}

// --- Tree ---

func treeSprite(v uint) Sprite {
	bgR, bgG, bgB := uint8(22), uint8(55), uint8(22)
	s := FillSprite(' ', 0, 0, 0, bgR, bgG, bgB)

	leafR, leafG, leafB := uint8(35), uint8(160), uint8(35)
	darkR, darkG, darkB := uint8(25), uint8(120), uint8(25)
	trunkR, trunkG, trunkB := uint8(110), uint8(75), uint8(30)

	leafG += uint8(v * 5)

	// Canopy (rows 0-2), trunk (rows 3-4)
	canopy := [3]struct{ start, end int }{
		{3, 7}, // row 0
		{2, 8}, // row 1
		{3, 7}, // row 2
	}

	leafChars := []rune{'♣', '♠', '♣', '♠'}

	for row := 0; row < 3; row++ {
		c := canopy[row]
		for x := c.start; x < c.end; x++ {
			ch := leafChars[(x+row+int(v))%len(leafChars)]
			lr, lg, lb := leafR, leafG, leafB
			if x == c.start || x == c.end-1 {
				lr, lg, lb = darkR, darkG, darkB
			}
			s[row][x] = SCBold(ch, lr, lg, lb, bgR, bgG, bgB)
		}
	}

	s[3][4] = SC('║', trunkR, trunkG, trunkB, bgR, bgG, bgB)
	s[3][5] = SC('║', trunkR-10, trunkG-10, trunkB-5, bgR, bgG, bgB)
	s[4][4] = SC('║', trunkR, trunkG, trunkB, bgR, bgG, bgB)
	s[4][5] = SC('║', trunkR-10, trunkG-10, trunkB-5, bgR, bgG, bgB)

	return s
}

// --- Path ---

func pathSprite(v uint) Sprite {
	bgR, bgG, bgB := uint8(120), uint8(95), uint8(55)
	fgR, fgG, fgB := uint8(150), uint8(120), uint8(75)

	s := FillSprite(' ', fgR, fgG, fgB, bgR, bgG, bgB)

	// Worn center (row 2)
	for x := 2; x < 8; x++ {
		s[2][x] = SC(' ', fgR, fgG, fgB, bgR+8, bgG+6, bgB+4)
	}

	type pebble struct{ x, y int }
	pebbles := [4][]pebble{
		{{1, 0}, {5, 1}, {8, 3}, {3, 4}},
		{{2, 1}, {7, 3}, {9, 0}, {0, 4}},
		{{0, 0}, {4, 2}, {8, 4}, {6, 1}},
		{{3, 0}, {9, 2}, {1, 3}, {6, 4}},
	}

	for _, p := range pebbles[v] {
		s[p.y][p.x] = SC('·', fgR, fgG, fgB, bgR, bgG, bgB)
	}

	return s
}

// --- Door ---

func doorSprite() Sprite {
	bgR, bgG, bgB := uint8(110), uint8(75), uint8(30)
	frameR, frameG, frameB := uint8(80), uint8(55), uint8(20)
	plankR, plankG, plankB := uint8(140), uint8(100), uint8(40)
	knobR, knobG, knobB := uint8(210), uint8(170), uint8(60)

	s := FillSprite(' ', plankR, plankG, plankB, bgR, bgG, bgB)

	// Header beam (row 0)
	for x := 0; x < TileWidth; x++ {
		s[0][x] = SCBold('▀', frameR+30, frameG+20, frameB+10, frameR, frameG, frameB)
	}

	// Frame pillars (cols 0 and 9)
	for y := 1; y < TileHeight; y++ {
		s[y][0] = SC('║', frameR+20, frameG+15, frameB+5, frameR, frameG, frameB)
		s[y][9] = SC('║', frameR+20, frameG+15, frameB+5, frameR, frameG, frameB)
	}

	// Plank lines
	for y := 1; y < TileHeight; y++ {
		s[y][3] = SC('│', plankR-20, plankG-15, plankB-10, bgR, bgG, bgB)
		s[y][6] = SC('│', plankR-20, plankG-15, plankB-10, bgR, bgG, bgB)
	}

	// Doorknob (row 2, col 7)
	s[2][7] = SCBold('●', knobR, knobG, knobB, bgR, bgG, bgB)

	return s
}

// --- Floor ---

func floorSprite(v uint) Sprite {
	bgR, bgG, bgB := uint8(72), uint8(52), uint8(32)
	fgR, fgG, fgB := uint8(92), uint8(68), uint8(42)

	s := FillSprite(' ', fgR, fgG, fgB, bgR, bgG, bgB)

	// Plank lines at rows 0 and 3
	for _, row := range []int{0, 3} {
		for x := 0; x < TileWidth; x++ {
			s[row][x] = SC('─', fgR, fgG, fgB, bgR, bgG, bgB)
		}
	}

	type grain struct{ x, y int }
	grains := [4][]grain{
		{{2, 2}, {7, 4}},
		{{4, 1}, {8, 4}},
		{{1, 1}, {6, 4}},
		{{3, 2}, {5, 1}},
	}
	for _, g := range grains[v] {
		s[g.y][g.x] = SC('·', fgR-10, fgG-10, fgB-5, bgR, bgG, bgB)
	}

	return s
}

// --- Fence ---

func fenceSprite(v uint) Sprite {
	bgR, bgG, bgB := uint8(28), uint8(65), uint8(28)
	fgR, fgG, fgB := uint8(155), uint8(115), uint8(55)

	s := FillSprite(' ', 0, 0, 0, bgR, bgG, bgB)

	// Posts at cols 1, 5, 9
	for _, col := range []int{1, 5, 9} {
		for y := 1; y < 4; y++ {
			s[y][col] = SCBold('║', fgR, fgG, fgB, bgR, bgG, bgB)
		}
	}

	// Rails at rows 1 and 3
	for _, row := range []int{1, 3} {
		for x := 0; x < TileWidth; x++ {
			if x == 1 || x == 5 || x == 9 {
				continue
			}
			s[row][x] = SC('═', fgR-10, fgG-10, fgB-5, bgR, bgG, bgB)
		}
	}

	if v%2 == 0 {
		s[4][3] = SC(',', 50, 115, 42, bgR, bgG, bgB)
		s[4][7] = SC('.', 60, 135, 50, bgR, bgG, bgB)
	}

	return s
}

// --- Fallback ---

func fallbackSprite(tile maps.TileDef) Sprite {
	fgR, fgG, fgB := AnsiToRGB(tile.Fg)
	bgR, bgG, bgB := uint8(10), uint8(10), uint8(15)
	if tile.Bg > 0 {
		bgR, bgG, bgB = AnsiToRGB(tile.Bg)
	}

	s := FillSprite(' ', fgR, fgG, fgB, bgR, bgG, bgB)
	s[TileHeight/2][TileWidth/2] = SC(tile.Char, fgR, fgG, fgB, bgR, bgG, bgB)
	return s
}

func min8(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}
