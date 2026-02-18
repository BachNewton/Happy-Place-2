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

	frames      int // max frame count
	hasBase     bool
	hasDY       map[int]bool // which DY values exist
	isConnected bool
}

// SpriteRegistry holds all loaded pixel sprites.
type SpriteRegistry struct {
	tiles   map[string]*tileData
	players [6][4]PixelSprite // [color][dir]
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
			sprites:   make(map[int]PixelSprite),
			parts:     make(map[string]map[int]PixelSprite),
			connected: make(map[string]PixelSprite),
			hasDY:     make(map[int]bool),
		}
		reg.tiles[tileName] = td
	}

	remaining := parts[varIdx+1:]

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
