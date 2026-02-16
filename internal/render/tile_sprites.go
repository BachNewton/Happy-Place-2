package render

import "happy-place-2/internal/maps"

// tileFunc generates a sprite for a tile at world position (wx,wy) at the given tick.
type tileFunc func(wx, wy int, tick uint64, m *maps.Map) Sprite

// tileEntry holds a named tile's sprite generator and its variant count.
type tileEntry struct {
	name      string
	fn        tileFunc
	variants  int // number of distinct variants (1 = no variation)
	connected bool
	connFn    func(mask uint8, v uint, tick uint64) Sprite
}

// TileHash maps world coordinates to a deterministic pseudo-random value.
// All tiles must use this (via % variants) for variant selection so the
// debug view can automatically find coordinates for every variant.
func TileHash(wx, wy int) uint {
	return uint(wx*7 + wy*13)
}

// variantCoord returns a (wx, wy) pair that produces the given variant index
// when passed through TileHash(wx, wy) % variants.
func variantCoord(v, variants int) (int, int) {
	for wy := 0; wy < 100; wy++ {
		for wx := 0; wx < 100; wx++ {
			if int(TileHash(wx, wy))%variants == v {
				return wx, wy
			}
		}
	}
	return 0, 0
}

// variantTile builds a tileEntry for tiles whose appearance depends only
// on a variant index and the tick (the common case).
func variantTile(name string, n int, fn func(v uint, tick uint64) Sprite) tileEntry {
	return tileEntry{
		name: name,
		fn: func(wx, wy int, tick uint64, m *maps.Map) Sprite {
			return fn(TileHash(wx, wy)%uint(n), tick)
		},
		variants: n,
	}
}

// posVariantTile builds a tileEntry for tiles that also need world position
// beyond variant selection (e.g., wall mortar line offsets).
func posVariantTile(name string, n int, fn func(wx, wy int, v uint, tick uint64) Sprite) tileEntry {
	return tileEntry{
		name: name,
		fn: func(wx, wy int, tick uint64, m *maps.Map) Sprite {
			return fn(wx, wy, TileHash(wx, wy)%uint(n), tick)
		},
		variants: n,
	}
}

// Connection bitmask constants for connected tiles.
const (
	ConnN uint8 = 1
	ConnE uint8 = 2
	ConnS uint8 = 4
	ConnW uint8 = 8
)

// neighborMask computes a 4-bit bitmask of same-name cardinal neighbors.
func neighborMask(name string, wx, wy int, m *maps.Map) uint8 {
	if m == nil {
		return 0
	}
	var mask uint8
	if m.TileAt(wx, wy-1).Name == name {
		mask |= ConnN
	}
	if m.TileAt(wx+1, wy).Name == name {
		mask |= ConnE
	}
	if m.TileAt(wx, wy+1).Name == name {
		mask |= ConnS
	}
	if m.TileAt(wx-1, wy).Name == name {
		mask |= ConnW
	}
	return mask
}

// connectedTile builds a tileEntry for tiles that adapt based on same-name neighbors.
func connectedTile(name string, n int, fn func(mask uint8, v uint, tick uint64) Sprite) tileEntry {
	return tileEntry{
		name: name,
		fn: func(wx, wy int, tick uint64, m *maps.Map) Sprite {
			mask := neighborMask(name, wx, wy, m)
			return fn(mask, TileHash(wx, wy)%uint(n), tick)
		},
		variants:  n,
		connected: true,
		connFn:    fn,
	}
}

// tileList is the single source of truth for all tile types.
// Order here determines debug view order. Names must be unique.
var tileList = []tileEntry{
	variantTile("grass", 4, func(v uint, tick uint64) Sprite { return grassSprite(v, tick) }),
	posVariantTile("wall", 4, func(wx, wy int, v uint, _ uint64) Sprite { return wallSprite(wx, wy, v) }),
	posVariantTile("water", 1, func(wx, wy int, _ uint, tick uint64) Sprite { return waterSprite(wx, wy, tick) }),
	variantTile("tree", 4, func(v uint, _ uint64) Sprite { return treeSprite(v) }),
	variantTile("path", 4, func(v uint, _ uint64) Sprite { return pathSprite(v) }),
	variantTile("door", 1, func(_ uint, _ uint64) Sprite { return doorSprite() }),
	variantTile("floor", 4, func(v uint, _ uint64) Sprite { return floorSprite(v) }),
	connectedTile("fence", 2, func(mask uint8, v uint, _ uint64) Sprite { return fenceSprite(mask, v) }),
	variantTile("flowers", 6, func(v uint, _ uint64) Sprite { return flowerSprite(v) }),
}

// tileIndex maps tile names to entries for O(1) lookup. Built in init().
var tileIndex map[string]*tileEntry

func init() {
	tileIndex = make(map[string]*tileEntry, len(tileList))
	for i := range tileList {
		name := tileList[i].name
		if _, exists := tileIndex[name]; exists {
			panic("duplicate tile name: " + name)
		}
		tileIndex[name] = &tileList[i]
	}
}

// TileSprite returns the sprite for a tile at world position (wx,wy) at the given tick.
func TileSprite(tile maps.TileDef, wx, wy int, tick uint64, m *maps.Map) Sprite {
	if e, ok := tileIndex[tile.Name]; ok {
		return e.fn(wx, wy, tick, m)
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

func wallSprite(wx, wy int, v uint) Sprite {
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

func fenceSprite(mask uint8, v uint) Sprite {
	bgR, bgG, bgB := uint8(28), uint8(65), uint8(28)
	fgR, fgG, fgB := uint8(155), uint8(115), uint8(55)
	railR, railG, railB := fgR-10, fgG-10, fgB-5

	s := FillSprite(' ', 0, 0, 0, bgR, bgG, bgB)

	// Center post always present (cols 4-5, rows 1-3)
	for y := 1; y <= 3; y++ {
		s[y][4] = SCBold('║', fgR, fgG, fgB, bgR, bgG, bgB)
		s[y][5] = SCBold('║', fgR, fgG, fgB, bgR, bgG, bgB)
	}

	// North connection: extend vertical rails to row 0
	if mask&ConnN != 0 {
		s[0][4] = SCBold('║', fgR, fgG, fgB, bgR, bgG, bgB)
		s[0][5] = SCBold('║', fgR, fgG, fgB, bgR, bgG, bgB)
	}

	// South connection: extend vertical rails to row 4
	if mask&ConnS != 0 {
		s[4][4] = SCBold('║', fgR, fgG, fgB, bgR, bgG, bgB)
		s[4][5] = SCBold('║', fgR, fgG, fgB, bgR, bgG, bgB)
	}

	// East connection: horizontal rails cols 6-9
	if mask&ConnE != 0 {
		for x := 6; x <= 9; x++ {
			s[1][x] = SC('═', railR, railG, railB, bgR, bgG, bgB)
			s[3][x] = SC('═', railR, railG, railB, bgR, bgG, bgB)
		}
	}

	// West connection: horizontal rails cols 0-3
	if mask&ConnW != 0 {
		for x := 0; x <= 3; x++ {
			s[1][x] = SC('═', railR, railG, railB, bgR, bgG, bgB)
			s[3][x] = SC('═', railR, railG, railB, bgR, bgG, bgB)
		}
	}

	// Grass tuft decoration only when no south connection
	if mask&ConnS == 0 && v%2 == 0 {
		s[4][3] = SC(',', 50, 115, 42, bgR, bgG, bgB)
		s[4][7] = SC('.', 60, 135, 50, bgR, bgG, bgB)
	}

	return s
}

// --- Flowers ---

func flowerSprite(v uint) Sprite {
	bgR, bgG, bgB := uint8(28), uint8(65), uint8(28)

	s := FillSprite(' ', 0, 0, 0, bgR, bgG, bgB)

	// 6 flower color palettes
	type flowerColor struct {
		r, g, b uint8
		ch      rune
	}
	palettes := [6]flowerColor{
		{230, 60, 70, '*'},   // red
		{240, 200, 50, '*'},  // yellow
		{180, 80, 200, '*'},  // purple
		{240, 140, 50, '@'},  // orange
		{230, 130, 170, '@'}, // pink
		{100, 170, 230, '@'}, // blue
	}

	stemR, stemG, stemB := uint8(50), uint8(130), uint8(40)

	// Each variant gets a different primary color and flower arrangement
	fc := palettes[v]
	// Secondary color from a neighboring palette
	fc2 := palettes[(v+3)%6]

	type flower struct{ x, y int }
	arrangements := [6][]flower{
		{{2, 1}, {7, 0}, {4, 3}, {9, 2}},
		{{1, 0}, {5, 2}, {8, 4}, {3, 1}},
		{{0, 2}, {6, 0}, {3, 4}, {8, 1}},
		{{4, 0}, {1, 3}, {7, 2}, {9, 4}},
		{{2, 0}, {6, 3}, {0, 4}, {8, 1}},
		{{5, 1}, {1, 4}, {9, 0}, {3, 2}},
	}

	for i, f := range arrangements[v] {
		// Stem below flower (if room)
		if f.y+1 < TileHeight {
			s[f.y+1][f.x] = SC('|', stemR, stemG, stemB, bgR, bgG, bgB)
		}
		// Alternate primary and secondary colors
		c := fc
		if i%2 == 1 {
			c = fc2
		}
		s[f.y][f.x] = SCBold(c.ch, c.r, c.g, c.b, bgR, bgG, bgB)
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

// --- Sign overlay ---

// SignSprite returns a sprite overlay for a sign mounted on a wall.
// Transparent cells let the wall show through.
func SignSprite() Sprite {
	T := TransparentCell

	boardR, boardG, boardB := uint8(140), uint8(100), uint8(50)
	edgeR, edgeG, edgeB := boardR-30, boardG-25, boardB-15
	textR, textG, textB := uint8(200), uint8(180), uint8(120)

	var s Sprite
	for y := 0; y < TileHeight; y++ {
		for x := 0; x < TileWidth; x++ {
			s[y][x] = T()
		}
	}

	// Row 1: top edge ┌──────┐
	s[1][1] = SC('┌', boardR, boardG, boardB, edgeR, edgeG, edgeB)
	for x := 2; x <= 7; x++ {
		s[1][x] = SC('─', boardR, boardG, boardB, edgeR, edgeG, edgeB)
	}
	s[1][8] = SC('┐', boardR, boardG, boardB, edgeR, edgeG, edgeB)

	// Row 2: sign face │ ≡≡≡≡ │
	s[2][1] = SC('│', boardR, boardG, boardB, edgeR, edgeG, edgeB)
	s[2][2] = SC(' ', textR, textG, textB, edgeR, edgeG, edgeB)
	for x := 3; x <= 6; x++ {
		s[2][x] = SC('≡', textR, textG, textB, edgeR, edgeG, edgeB)
	}
	s[2][7] = SC(' ', textR, textG, textB, edgeR, edgeG, edgeB)
	s[2][8] = SC('│', boardR, boardG, boardB, edgeR, edgeG, edgeB)

	// Row 3: bottom edge └──────┘
	s[3][1] = SC('└', boardR, boardG, boardB, edgeR, edgeG, edgeB)
	for x := 2; x <= 7; x++ {
		s[3][x] = SC('─', boardR, boardG, boardB, edgeR, edgeG, edgeB)
	}
	s[3][8] = SC('┘', boardR, boardG, boardB, edgeR, edgeG, edgeB)

	return s
}
