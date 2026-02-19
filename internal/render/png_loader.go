package render

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// LoadPixelSprite reads a 16x16 PNG and returns a PixelSprite.
// Alpha=0 or magenta (#FF00FF) pixels are treated as transparent.
func LoadPixelSprite(path string) (PixelSprite, error) {
	var ps PixelSprite

	f, err := os.Open(path)
	if err != nil {
		return ps, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return ps, fmt.Errorf("decode %s: %w", path, err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != PixelTileW || bounds.Dy() != PixelTileH {
		return ps, fmt.Errorf("%s: expected %dx%d, got %dx%d", path, PixelTileW, PixelTileH, bounds.Dx(), bounds.Dy())
	}

	for y := 0; y < PixelTileH; y++ {
		for x := 0; x < PixelTileW; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

			if a < 0x8000 || (r8 == 0xFF && g8 == 0x00 && b8 == 0xFF) {
				ps[y][x] = TransparentPixel()
			} else {
				ps[y][x] = P(r8, g8, b8)
			}
		}
	}

	return ps, nil
}

// tileData holds all loaded sprites for a single tile type.
type tileData struct {
	// For simple/animated tiles: frame -> PixelSprite
	sprites map[int]PixelSprite
	// For tall tiles: part name -> frame -> PixelSprite
	parts map[string]map[int]PixelSprite
	// For connected tiles: mask -> PixelSprite
	connected map[string]PixelSprite
	// For blob tiles: part name -> sprite (13 named parts)
	blob map[string]PixelSprite
	// For blob tiles: 8-bit mask -> precomputed composite sprite
	blobComposite map[uint8]PixelSprite
	// For border blob tiles: 8-bit mask -> precomputed composite sprite (rendered on neighbors)
	blobBorderComposite map[uint8]PixelSprite

	frames      int // max frame count
	hasBase     bool
	hasDY       map[int]bool // which DY values exist
	isConnected  bool
	isBlob       bool
	isBorderBlob bool
}

// SpriteRegistry holds all loaded pixel sprites.
type SpriteRegistry struct {
	tiles          map[string]*tileData
	players        [6][4]PixelSprite // [color][dir]
	borderBlobTiles []string          // tile names that use border blob rendering
}

// NewSpriteRegistry loads all PNGs from the given directory.
func NewSpriteRegistry(spritesDir string) (*SpriteRegistry, error) {
	reg := &SpriteRegistry{
		tiles: make(map[string]*tileData),
	}

	tilesDir := filepath.Join(spritesDir, "tiles")
	playersDir := filepath.Join(spritesDir, "players")

	// Load tile sprites
	if err := reg.loadTiles(tilesDir); err != nil {
		return nil, fmt.Errorf("load tiles: %w", err)
	}

	// Generate blob composites for all blob tile types
	reg.generateBlobComposites()
	reg.generateBorderBlobComposites()

	// Load player sprites and generate palette swaps
	if err := reg.loadPlayers(playersDir); err != nil {
		return nil, fmt.Errorf("load players: %w", err)
	}

	return reg, nil
}

// loadTiles scans the tiles directory and loads all PNGs.
// Naming conventions:
//   - Simple: grass_0.png (tile_variant.png)
//   - Animated: water_0_f0.png (tile_variant_fN.png)
//   - Tall: tree_0_base.png, tree_0_dy1.png, tree_0_dy2.png
//   - Connected: fence_0_0000.png through fence_0_1111.png
func (reg *SpriteRegistry) loadTiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read tiles dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".png") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".png")
		path := filepath.Join(dir, entry.Name())

		sprite, err := LoadPixelSprite(path)
		if err != nil {
			log.Printf("Warning: skipping %s: %v", path, err)
			continue
		}

		reg.parseTileSprite(name, sprite)
	}

	return nil
}

// parseTileSprite parses a filename and stores the sprite in the registry.
// Filenames follow the pattern: tilename_0[_suffix].png where 0 is the variant number (always 0).
func (reg *SpriteRegistry) parseTileSprite(name string, sprite PixelSprite) {
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return
	}

	// Find the variant index (first numeric-only part from the end)
	varIdx := -1
	for i := len(parts) - 1; i >= 1; i-- {
		if _, err := strconv.Atoi(parts[i]); err == nil {
			varIdx = i
			break
		}
		if strings.HasPrefix(parts[i], "f") {
			if _, err := strconv.Atoi(parts[i][1:]); err == nil {
				continue
			}
		}
		if parts[i] == "base" || strings.HasPrefix(parts[i], "dy") {
			continue
		}
		if parts[i] == "blob" || parts[i] == "edge" || parts[i] == "outer" || parts[i] == "inner" || parts[i] == "center" {
			continue
		}
		// Cardinal/ordinal suffixes used in blob part names
		if parts[i] == "n" || parts[i] == "s" || parts[i] == "e" || parts[i] == "w" ||
			parts[i] == "nw" || parts[i] == "ne" || parts[i] == "sw" || parts[i] == "se" {
			continue
		}
		if len(parts[i]) == 4 && isConnectionMask(parts[i]) {
			continue
		}
		break
	}

	if varIdx == -1 {
		return
	}

	tileName := strings.Join(parts[:varIdx], "_")

	td := reg.tiles[tileName]
	if td == nil {
		td = &tileData{
			sprites:             make(map[int]PixelSprite),
			parts:               make(map[string]map[int]PixelSprite),
			connected:           make(map[string]PixelSprite),
			blob:                make(map[string]PixelSprite),
			blobComposite:       make(map[uint8]PixelSprite),
			blobBorderComposite: make(map[uint8]PixelSprite),
			hasDY:               make(map[int]bool),
		}
		reg.tiles[tileName] = td
	}

	remaining := parts[varIdx+1:]

	// Blob tile: water_0_blob_edge_n.png -> partName = "edge_n"
	if len(remaining) >= 2 && remaining[0] == "blob" {
		partName := strings.Join(remaining[1:], "_")
		td.isBlob = true
		td.blob[partName] = sprite
		return
	}

	if len(remaining) == 0 {
		// Simple: grass_0.png
		td.sprites[0] = sprite
		if td.frames < 1 {
			td.frames = 1
		}
		return
	}

	// Check for animation frame
	frame := 0
	hasFrame := false
	for i, p := range remaining {
		if strings.HasPrefix(p, "f") {
			if f, err := strconv.Atoi(p[1:]); err == nil {
				frame = f
				hasFrame = true
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}
	}

	if hasFrame && len(remaining) == 0 {
		// Animated: water_0_f0.png
		td.sprites[frame] = sprite
		if frame+1 > td.frames {
			td.frames = frame + 1
		}
		return
	}

	if len(remaining) == 1 {
		part := remaining[0]

		// Connected: fence_0_0000.png
		if len(part) == 4 && isConnectionMask(part) {
			td.isConnected = true
			td.connected[part] = sprite
			return
		}

		// Tall base: tree_0_base.png
		if part == "base" {
			td.hasBase = true
			partMap := td.parts["base"]
			if partMap == nil {
				partMap = make(map[int]PixelSprite)
				td.parts["base"] = partMap
			}
			partMap[frame] = sprite
			return
		}

		// Tall DY: tree_0_dy1.png
		if strings.HasPrefix(part, "dy") {
			if dy, err := strconv.Atoi(part[2:]); err == nil {
				td.hasDY[dy] = true
				partMap := td.parts[part]
				if partMap == nil {
					partMap = make(map[int]PixelSprite)
					td.parts[part] = partMap
				}
				partMap[frame] = sprite
				return
			}
		}
	}
}

func isConnectionMask(s string) bool {
	for _, c := range s {
		if c != '0' && c != '1' {
			return false
		}
	}
	return true
}

// GetTileSprites returns the PixelTileSprites for a tile at the given tick.
func (reg *SpriteRegistry) GetTileSprites(tileName string, tick uint64) PixelTileSprites {
	td := reg.tiles[tileName]
	if td == nil {
		return PixelTileSprites{Base: FillPixelSprite(255, 0, 255)} // magenta = missing
	}

	frameCount := td.frames
	if frameCount < 1 {
		frameCount = 1
	}
	frame := int(tick/8) % frameCount

	if td.hasBase {
		// Tall tile
		base := reg.getTilePart(td, "base", frame)
		var overlays []PixelOverlay
		dyValues := make([]int, 0, len(td.hasDY))
		for dy := range td.hasDY {
			dyValues = append(dyValues, dy)
		}
		sort.Ints(dyValues)
		for _, dy := range dyValues {
			partName := fmt.Sprintf("dy%d", dy)
			s := reg.getTilePart(td, partName, frame)
			overlays = append(overlays, PixelOverlay{Sprite: s, DY: dy})
		}
		return PixelTileSprites{Base: base, Overlays: overlays}
	}

	// Simple/animated tile
	if s, ok := td.sprites[frame]; ok {
		return PixelTileSprites{Base: s}
	}
	// Fallback to frame 0
	if s, ok := td.sprites[0]; ok {
		return PixelTileSprites{Base: s}
	}

	return PixelTileSprites{Base: FillPixelSprite(255, 0, 255)}
}

// GetConnectedTileSprite returns a sprite for a connected tile with the given neighbor mask.
func (reg *SpriteRegistry) GetConnectedTileSprite(tileName string, mask uint8) PixelSprite {
	td := reg.tiles[tileName]
	if td == nil {
		return FillPixelSprite(255, 0, 255)
	}

	maskStr := fmt.Sprintf("%d%d%d%d",
		boolToInt(mask&ConnN != 0),
		boolToInt(mask&ConnE != 0),
		boolToInt(mask&ConnS != 0),
		boolToInt(mask&ConnW != 0),
	)

	if s, ok := td.connected[maskStr]; ok {
		return s
	}

	// Fallback: try 0000
	if s, ok := td.connected["0000"]; ok {
		return s
	}

	return FillPixelSprite(255, 0, 255)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (reg *SpriteRegistry) getTilePart(td *tileData, partName string, frame int) PixelSprite {
	partMap := td.parts[partName]
	if partMap == nil {
		return FillPixelSprite(255, 0, 255)
	}
	if s, ok := partMap[frame]; ok {
		return s
	}
	if s, ok := partMap[0]; ok {
		return s
	}
	return FillPixelSprite(255, 0, 255)
}

// HasTile returns whether the registry has sprites for the given tile name.
func (reg *SpriteRegistry) HasTile(name string) bool {
	_, ok := reg.tiles[name]
	return ok
}

// TileIsConnected returns whether a tile type uses connected sprites.
func (reg *SpriteRegistry) TileIsConnected(name string) bool {
	td := reg.tiles[name]
	if td == nil {
		return false
	}
	return td.isConnected
}

// TileIsBlob returns whether a tile type uses blob (autotile) sprites.
func (reg *SpriteRegistry) TileIsBlob(name string) bool {
	td := reg.tiles[name]
	if td == nil {
		return false
	}
	return td.isBlob
}

// GetBlobTileSprite returns a precomputed composite sprite for the given 8-bit neighbor mask.
func (reg *SpriteRegistry) GetBlobTileSprite(tileName string, mask uint8) PixelSprite {
	td := reg.tiles[tileName]
	if td == nil {
		return FillPixelSprite(255, 0, 255)
	}
	if s, ok := td.blobComposite[mask]; ok {
		return s
	}
	// Fallback to center
	if s, ok := td.blob["center"]; ok {
		return s
	}
	return FillPixelSprite(255, 0, 255)
}

// blobMaskToParts returns the blob part names needed for a given 8-bit mask.
// Returns a single part for simple cases, or multiple inner corner parts
// that need to be composited onto the center.
func blobMaskToParts(mask uint8) []string {
	n := mask&BlobN != 0
	e := mask&BlobE != 0
	s := mask&BlobS != 0
	w := mask&BlobW != 0
	ne := mask&BlobNE != 0
	se := mask&BlobSE != 0
	sw := mask&BlobSW != 0
	nw := mask&BlobNW != 0

	// Count missing cardinals
	missingCardinals := 0
	if !n {
		missingCardinals++
	}
	if !e {
		missingCardinals++
	}
	if !s {
		missingCardinals++
	}
	if !w {
		missingCardinals++
	}

	switch {
	case missingCardinals >= 3:
		// Peninsula or isolated — use center as fallback
		return []string{"center"}

	case missingCardinals == 2:
		// Two adjacent missing cardinals → outer corner
		// Two opposite missing → use center fallback
		if !n && !w {
			return []string{"outer_nw"}
		}
		if !n && !e {
			return []string{"outer_ne"}
		}
		if !s && !w {
			return []string{"outer_sw"}
		}
		if !s && !e {
			return []string{"outer_se"}
		}
		// Opposite missing (N+S or E+W) — center fallback
		return []string{"center"}

	case missingCardinals == 1:
		// One cardinal missing → edge
		if !n {
			return []string{"edge_n"}
		}
		if !e {
			return []string{"edge_e"}
		}
		if !s {
			return []string{"edge_s"}
		}
		if !w {
			return []string{"edge_w"}
		}

	default:
		// All cardinals present — check diagonals for inner corners
		var parts []string
		if !nw {
			parts = append(parts, "inner_nw")
		}
		if !ne {
			parts = append(parts, "inner_ne")
		}
		if !sw {
			parts = append(parts, "inner_sw")
		}
		if !se {
			parts = append(parts, "inner_se")
		}
		if len(parts) == 0 {
			return []string{"center"}
		}
		return parts
	}

	return []string{"center"}
}

// generateBlobComposites pre-generates all 256 possible blob tile masks.
func (reg *SpriteRegistry) generateBlobComposites() {
	for _, td := range reg.tiles {
		if !td.isBlob {
			continue
		}

		center, hasCenter := td.blob["center"]
		if !hasCenter {
			continue
		}

		for mask := 0; mask < 256; mask++ {
			m := uint8(mask)
			parts := blobMaskToParts(m)

			if len(parts) == 1 {
				if s, ok := td.blob[parts[0]]; ok {
					td.blobComposite[m] = s
				} else {
					td.blobComposite[m] = center
				}
				continue
			}

			// Multi-part composite: start with center, overlay inner corners
			composite := center
			for _, partName := range parts {
				inner, ok := td.blob[partName]
				if !ok {
					continue
				}
				// Overlay: where inner differs from center, use inner pixel
				for y := 0; y < PixelTileH; y++ {
					for x := 0; x < PixelTileW; x++ {
						ip := inner[y][x]
						cp := center[y][x]
						if ip != cp {
							composite[y][x] = ip
						}
					}
				}
			}
			td.blobComposite[m] = composite
		}
	}
}

// generateBorderBlobComposites marks border blob tiles and pre-generates
// all 256 possible border composites using flipped edge/corner mapping.
func (reg *SpriteRegistry) generateBorderBlobComposites() {
	borderBlobNames := []string{"path"}
	for _, name := range borderBlobNames {
		td := reg.tiles[name]
		if td == nil || !td.isBlob {
			continue
		}
		td.isBorderBlob = true
		reg.borderBlobTiles = append(reg.borderBlobTiles, name)
	}

	for _, name := range reg.borderBlobTiles {
		td := reg.tiles[name]
		center, hasCenter := td.blob["center"]
		if !hasCenter {
			continue
		}

		for mask := 0; mask < 256; mask++ {
			m := uint8(mask)
			parts := borderBlobMaskToParts(m)
			if parts == nil {
				continue
			}

			if len(parts) == 1 {
				if s, ok := td.blob[parts[0]]; ok {
					td.blobBorderComposite[m] = s
				} else {
					td.blobBorderComposite[m] = center
				}
				continue
			}

			// Multi-part composite (inner corners): start with center, overlay
			composite := center
			for _, partName := range parts {
				inner, ok := td.blob[partName]
				if !ok {
					continue
				}
				for y := 0; y < PixelTileH; y++ {
					for x := 0; x < PixelTileW; x++ {
						ip := inner[y][x]
						cp := center[y][x]
						if ip != cp {
							composite[y][x] = ip
						}
					}
				}
			}
			td.blobBorderComposite[m] = composite
		}
	}
}

// TileIsBorderBlob returns whether a tile type uses border blob rendering
// (transitions render on neighboring tiles, not on the tile itself).
func (reg *SpriteRegistry) TileIsBorderBlob(name string) bool {
	td := reg.tiles[name]
	if td == nil {
		return false
	}
	return td.isBorderBlob
}

// BorderBlobNames returns the list of border blob tile names.
func (reg *SpriteRegistry) BorderBlobNames() []string {
	return reg.borderBlobTiles
}

// GetBorderBlobTileSprite returns a precomputed border blob composite
// for the given 8-bit neighbor mask.
func (reg *SpriteRegistry) GetBorderBlobTileSprite(tileName string, mask uint8) PixelSprite {
	td := reg.tiles[tileName]
	if td == nil {
		return FillPixelSprite(255, 0, 255)
	}
	if s, ok := td.blobBorderComposite[mask]; ok {
		return s
	}
	if s, ok := td.blob["center"]; ok {
		return s
	}
	return FillPixelSprite(255, 0, 255)
}

// GetBlobPartSprite returns a raw blob part sprite by name (e.g., "outer_nw").
func (reg *SpriteRegistry) GetBlobPartSprite(tileName, partName string) (PixelSprite, bool) {
	td := reg.tiles[tileName]
	if td == nil {
		return PixelSprite{}, false
	}
	s, ok := td.blob[partName]
	return s, ok
}

// loadPlayers loads the 4 direction templates and generates palette swaps.
func (reg *SpriteRegistry) loadPlayers(dir string) error {
	dirNames := []string{"down", "up", "left", "right"}
	var templates [4]PixelSprite
	var loaded [4]bool

	for i, dName := range dirNames {
		path := filepath.Join(dir, "player_"+dName+".png")
		sprite, err := LoadPixelSprite(path)
		if err != nil {
			log.Printf("Warning: player sprite %s not found, using placeholder", path)
			templates[i] = FillPixelSprite(200, 60, 60) // red placeholder
			continue
		}
		templates[i] = sprite
		loaded[i] = true
	}

	// Generate 6 color variants x 4 directions
	for colorIdx := 0; colorIdx < 6; colorIdx++ {
		targetR := PlayerBGColors[colorIdx][0]
		targetG := PlayerBGColors[colorIdx][1]
		targetB := PlayerBGColors[colorIdx][2]

		pantR, pantG, pantB := targetR*2/3, targetG*2/3, targetB*2/3

		for dir := 0; dir < 4; dir++ {
			if !loaded[dir] {
				reg.players[colorIdx][dir] = FillPixelSprite(targetR, targetG, targetB)
				continue
			}

			var swapped PixelSprite
			for y := 0; y < PixelTileH; y++ {
				for x := 0; x < PixelTileW; x++ {
					px := templates[dir][y][x]
					if px.Transparent {
						swapped[y][x] = px
						continue
					}

					// Shirt: template red #FF0000 -> target color
					if px.R == 0xFF && px.G == 0x00 && px.B == 0x00 {
						swapped[y][x] = P(targetR, targetG, targetB)
					} else if px.R == 0xAA && px.G == 0x00 && px.B == 0x00 {
						// Pants: template dark red #AA0000 -> darkened target
						swapped[y][x] = P(pantR, pantG, pantB)
					} else {
						swapped[y][x] = px
					}
				}
			}
			reg.players[colorIdx][dir] = swapped
		}
	}

	return nil
}

// GetPlayerSprite returns the pixel sprite for a player with given direction and color.
func (reg *SpriteRegistry) GetPlayerSprite(dir, color int) PixelSprite {
	colorIdx := color % 6
	dirIdx := dir % 4
	return reg.players[colorIdx][dirIdx]
}
