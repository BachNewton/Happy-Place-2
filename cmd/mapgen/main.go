package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

// Tile indices for the wilderness legend.
const (
	tGrass        = 0
	tWater        = 1
	tTree         = 2
	tWall         = 3
	tFlowers      = 4
	tPath         = 5
	tSand         = 6
	tTallGrass    = 7
	tRock         = 8
	tShallowWater = 9
	tDirt         = 10
	tBridge       = 11
)

// jsonMap mirrors the on-disk format from internal/maps.
type jsonMap struct {
	Name    string              `json:"name"`
	Width   int                 `json:"width"`
	Height  int                 `json:"height"`
	Spawn   jsonSpawn           `json:"spawn"`
	Tiles   [][]int             `json:"tiles"`
	Legend  map[string]jsonTile `json:"legend"`
	Portals []interface{}       `json:"portals"`
}

type jsonSpawn struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type jsonTile struct {
	Char     string `json:"char"`
	Fg       string `json:"fg"`
	Walkable bool   `json:"walkable"`
	Name     string `json:"name"`
}

func main() {
	genType := flag.String("type", "", "generator type (wilderness)")
	seed := flag.Int64("seed", 0, "random seed (0 = random)")
	size := flag.String("size", "100x80", "map size as WxH")
	name := flag.String("name", "Wilderness", "map name")
	out := flag.String("out", "", "output file (default: stdout)")
	flag.Parse()

	if *genType == "" {
		fmt.Fprintln(os.Stderr, "Error: -type is required")
		fmt.Fprintln(os.Stderr, "Usage: mapgen -type wilderness [-seed N] [-size WxH] [-name Name] [-out file.json]")
		os.Exit(1)
	}

	if *genType != "wilderness" {
		fmt.Fprintf(os.Stderr, "Error: unknown generator type %q (available: wilderness)\n", *genType)
		os.Exit(1)
	}

	w, h, err := parseSize(*size)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if *seed == 0 {
		*seed = time.Now().UnixNano()
	}

	fmt.Fprintf(os.Stderr, "Generating %dx%d wilderness map %q (seed %d)...\n", w, h, *name, *seed)

	tiles := generateWilderness(w, h, *seed)

	spawnX, spawnY := findSpawn(tiles, w, h)
	fmt.Fprintf(os.Stderr, "Spawn: (%d, %d)\n", spawnX, spawnY)

	m := jsonMap{
		Name:   *name,
		Width:  w,
		Height: h,
		Spawn:  jsonSpawn{X: spawnX, Y: spawnY},
		Tiles:  tiles,
		Legend: map[string]jsonTile{
			"0":  {Char: ".", Fg: "green", Walkable: true, Name: "grass"},
			"1":  {Char: "~", Fg: "blue", Walkable: false, Name: "water"},
			"2":  {Char: "T", Fg: "green", Walkable: false, Name: "tree"},
			"3":  {Char: "#", Fg: "gray", Walkable: false, Name: "wall"},
			"4":  {Char: "*", Fg: "bright_red", Walkable: true, Name: "flowers"},
			"5":  {Char: ".", Fg: "yellow", Walkable: true, Name: "path"},
			"6":  {Char: "~", Fg: "yellow", Walkable: true, Name: "sand"},
			"7":  {Char: ";", Fg: "bright_green", Walkable: true, Name: "tall_grass"},
			"8":  {Char: "▒", Fg: "gray", Walkable: false, Name: "rock"},
			"9":  {Char: "~", Fg: "cyan", Walkable: true, Name: "shallow_water"},
			"10": {Char: ".", Fg: "yellow", Walkable: true, Name: "dirt"},
			"11": {Char: "=", Fg: "yellow", Walkable: true, Name: "bridge"},
		},
		Portals: []interface{}{},
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	if *out == "" {
		os.Stdout.Write(data)
		os.Stdout.WriteString("\n")
	} else {
		if err := os.WriteFile(*out, append(data, '\n'), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Wrote %s (%d bytes)\n", *out, len(data))
	}

	// Print tile distribution summary
	counts := make(map[int]int)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			counts[tiles[y][x]]++
		}
	}
	total := w * h
	names := map[int]string{
		0: "grass", 1: "water", 2: "tree", 3: "wall", 4: "flowers",
		5: "path", 6: "sand", 7: "tall_grass", 8: "rock", 9: "shallow_water",
		10: "dirt", 11: "bridge",
	}
	fmt.Fprintf(os.Stderr, "\nTile distribution:\n")
	for i := 0; i <= 11; i++ {
		if c, ok := counts[i]; ok {
			fmt.Fprintf(os.Stderr, "  %-15s %5d (%5.1f%%)\n", names[i], c, float64(c)/float64(total)*100)
		}
	}
}

func parseSize(s string) (int, int, error) {
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid size %q (expected WxH)", s)
	}
	w, err := strconv.Atoi(parts[0])
	if err != nil || w < 10 {
		return 0, 0, fmt.Errorf("invalid width %q (minimum 10)", parts[0])
	}
	h, err := strconv.Atoi(parts[1])
	if err != nil || h < 10 {
		return 0, 0, fmt.Errorf("invalid height %q (minimum 10)", parts[1])
	}
	return w, h, nil
}

func generateWilderness(w, h int, seed int64) [][]int {
	elevation := NewSimplexNoise(seed)
	moisture := NewSimplexNoise(seed + 1)
	detail := NewSimplexNoise(seed + 2)

	tiles := make([][]int, h)
	for y := 0; y < h; y++ {
		tiles[y] = make([]int, w)
		for x := 0; x < w; x++ {
			fx, fy := float64(x), float64(y)

			elev := elevation.Fractal(fx, fy, 0.02, 4, 2.0, 0.5)
			moist := moisture.Fractal(fx, fy, 0.03, 3, 2.0, 0.5)
			det := detail.Fractal(fx, fy, 0.1, 2, 2.0, 0.5)

			tiles[y][x] = classifyTile(elev, moist, det)
		}
	}

	// Edge treatment
	applyEdges(tiles, w, h, elevation)

	// Trail carving
	rng := rand.New(rand.NewSource(seed + 100))
	spawnX, spawnY := w/2, h/2
	// Find a walkable spot near center for trail start
	for r := 0; r < max(w, h)/2; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				nx, ny := spawnX+dx, spawnY+dy
				if nx > 0 && nx < w-1 && ny > 0 && ny < h-1 && isWalkable(tiles[ny][nx]) {
					spawnX, spawnY = nx, ny
					goto foundStart
				}
			}
		}
	}
foundStart:

	carveTrails(tiles, w, h, spawnX, spawnY, rng)

	// Ensure all walkable areas are reachable from spawn
	ensureConnectivity(tiles, w, h, spawnX, spawnY, rng)

	return tiles
}

func classifyTile(elev, moist, det float64) int {
	switch {
	case elev < 0.20:
		return tWater
	case elev < 0.28:
		return tShallowWater
	case elev < 0.32:
		return tSand
	case elev < 0.42:
		// Low plains
		if moist > 0.6 {
			return tFlowers
		}
		if moist > 0.45 {
			return tTallGrass
		}
		return tGrass
	case elev < 0.70:
		// Mid elevation
		if moist > 0.55 {
			return tTree
		}
		if moist > 0.35 {
			// Sparse mix using detail noise
			if det > 0.65 {
				return tTree
			}
			if det > 0.45 {
				return tTallGrass
			}
			return tGrass
		}
		return tGrass
	case elev < 0.78:
		return tRock
	default:
		return tWall
	}
}

func isWalkable(tile int) bool {
	switch tile {
	case tGrass, tFlowers, tPath, tSand, tTallGrass, tShallowWater, tDirt, tBridge:
		return true
	}
	return false
}

func applyEdges(tiles [][]int, w, h int, elevation *SimplexNoise) {
	borderDepth := 3

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Outermost ring is always impassable
			if x == 0 || x == w-1 || y == 0 || y == h-1 {
				elev := elevation.Fractal(float64(x), float64(y), 0.02, 4, 2.0, 0.5)
				if elev >= 0.70 {
					tiles[y][x] = tWall
				} else {
					tiles[y][x] = tTree
				}
				continue
			}

			// Border zone (inside outermost ring, up to borderDepth)
			dist := minOf(x, y, w-1-x, h-1-y)
			if dist < borderDepth {
				// Only convert walkable tiles in the border zone
				if isWalkable(tiles[y][x]) {
					elev := elevation.Fractal(float64(x), float64(y), 0.02, 4, 2.0, 0.5)
					// Use noise to shape the boundary — not a solid wall
					threshold := float64(borderDepth-dist) * 0.3
					noise := elevation.Fractal(float64(x)*2, float64(y)*2, 0.08, 2, 2.0, 0.5)
					if noise < threshold {
						if elev >= 0.65 {
							tiles[y][x] = tRock
						} else {
							tiles[y][x] = tTree
						}
					}
				}
			}
		}
	}
}

func minOf(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}

func carveTrails(tiles [][]int, w, h, startX, startY int, rng *rand.Rand) {
	// Generate 2-3 edge target points
	numTrails := 2 + rng.Intn(2)

	type point struct{ x, y int }
	targets := make([]point, numTrails)

	for i := 0; i < numTrails; i++ {
		switch rng.Intn(4) {
		case 0: // North edge
			targets[i] = point{borderClamp(rng.Intn(w), w), 1}
		case 1: // South edge
			targets[i] = point{borderClamp(rng.Intn(w), w), h - 2}
		case 2: // East edge
			targets[i] = point{w - 2, borderClamp(rng.Intn(h), h)}
		case 3: // West edge
			targets[i] = point{1, borderClamp(rng.Intn(h), h)}
		}
	}

	for _, target := range targets {
		carveTrail(tiles, w, h, startX, startY, target.x, target.y, rng)
	}
}

func borderClamp(v, limit int) int {
	if v < 4 {
		return 4
	}
	if v >= limit-4 {
		return limit - 5
	}
	return v
}

func carveTrail(tiles [][]int, w, h, sx, sy, tx, ty int, rng *rand.Rand) {
	x, y := sx, sy

	for steps := 0; steps < w*h; steps++ {
		if x == tx && y == ty {
			break
		}

		// Determine primary direction toward target
		dx, dy := 0, 0
		distX := tx - x
		distY := ty - y

		// Bias toward the axis with more distance
		if abs(distX) > abs(distY) {
			dx = sign(distX)
			// Random lateral drift
			if rng.Float64() < 0.3 {
				dy = sign(distY)
				if dy == 0 {
					dy = rng.Intn(2)*2 - 1
				}
				dx = 0
			}
		} else {
			dy = sign(distY)
			if rng.Float64() < 0.3 {
				dx = sign(distX)
				if dx == 0 {
					dx = rng.Intn(2)*2 - 1
				}
				dy = 0
			}
		}

		nx, ny := x+dx, y+dy
		if nx < 1 || nx >= w-1 || ny < 1 || ny >= h-1 {
			continue
		}

		// Place trail tile
		current := tiles[ny][nx]
		if current == tWater || current == tShallowWater {
			tiles[ny][nx] = tBridge
		} else if current != tPath && current != tBridge {
			tiles[ny][nx] = tPath

			// Place dirt alongside on grass/tall_grass neighbors
			for _, offset := range [][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				ax, ay := nx+offset[0], ny+offset[1]
				if ax >= 1 && ax < w-1 && ay >= 1 && ay < h-1 {
					adj := tiles[ay][ax]
					if adj == tGrass || adj == tTallGrass {
						if rng.Float64() < 0.4 {
							tiles[ay][ax] = tDirt
						}
					}
				}
			}
		}

		x, y = nx, ny
	}
}

func findSpawn(tiles [][]int, w, h int) (int, int) {
	cx, cy := w/2, h/2

	// Search outward from center for grass with mostly-walkable 3x3 neighborhood
	maxR := int(math.Max(float64(w), float64(h))) / 2
	for r := 0; r <= maxR; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if abs(dx) != r && abs(dy) != r {
					continue // only check the ring perimeter
				}
				x, y := cx+dx, cy+dy
				if x < 2 || x >= w-2 || y < 2 || y >= h-2 {
					continue
				}
				if tiles[y][x] != tGrass && tiles[y][x] != tPath {
					continue
				}
				// Check 3x3 neighborhood is mostly walkable
				walkCount := 0
				for ny := y - 1; ny <= y+1; ny++ {
					for nx := x - 1; nx <= x+1; nx++ {
						if isWalkable(tiles[ny][nx]) {
							walkCount++
						}
					}
				}
				if walkCount >= 7 {
					return x, y
				}
			}
		}
	}

	// Fallback: just find any walkable tile
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			if isWalkable(tiles[y][x]) {
				return x, y
			}
		}
	}
	return cx, cy
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

// --- Connectivity enforcement ---

type point struct{ x, y int }

// floodFill returns the set of walkable tiles reachable from (sx, sy).
func floodFill(tiles [][]int, w, h, sx, sy int) map[point]bool {
	region := make(map[point]bool)
	if !isWalkable(tiles[sy][sx]) {
		return region
	}

	stack := []point{{sx, sy}}
	region[point{sx, sy}] = true

	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
			nx, ny := p.x+d[0], p.y+d[1]
			if nx < 0 || nx >= w || ny < 0 || ny >= h {
				continue
			}
			np := point{nx, ny}
			if region[np] || !isWalkable(tiles[ny][nx]) {
				continue
			}
			region[np] = true
			stack = append(stack, np)
		}
	}
	return region
}

// ensureConnectivity finds disconnected walkable regions and connects them
// to the main (spawn-reachable) region. Small isolated pockets are filled in.
func ensureConnectivity(tiles [][]int, w, h, spawnX, spawnY int, rng *rand.Rand) {
	mainRegion := floodFill(tiles, w, h, spawnX, spawnY)

	// Find all walkable tiles NOT in the main region
	visited := make(map[point]bool)
	for p := range mainRegion {
		visited[p] = true
	}

	var islands []map[point]bool
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			p := point{x, y}
			if visited[p] || !isWalkable(tiles[y][x]) {
				continue
			}
			island := floodFill(tiles, w, h, x, y)
			for ip := range island {
				visited[ip] = true
			}
			islands = append(islands, island)
		}
	}

	if len(islands) == 0 {
		fmt.Fprintf(os.Stderr, "Connectivity: fully connected (%d walkable tiles)\n", len(mainRegion))
		return
	}

	totalOrphaned := 0
	for _, island := range islands {
		totalOrphaned += len(island)
	}
	fmt.Fprintf(os.Stderr, "Connectivity: %d reachable, %d orphaned across %d islands\n",
		len(mainRegion), totalOrphaned, len(islands))

	const fillThreshold = 15 // islands smaller than this get filled in

	connected := 0
	filled := 0
	for _, island := range islands {
		if len(island) < fillThreshold {
			// Fill tiny islands with trees — they'd just be confusing
			for p := range island {
				tiles[p.y][p.x] = tTree
			}
			filled += len(island)
			continue
		}

		// Connect larger islands: find closest pair of points between
		// this island and the main region, then carve a corridor
		carveConnection(tiles, w, h, mainRegion, island, rng)

		// Merge the island into the main region
		for p := range island {
			mainRegion[p] = true
		}
		// Also re-flood from the corridor to pick up newly connected tiles
		// (the carved corridor itself creates new walkable tiles)
		for p := range island {
			fresh := floodFill(tiles, w, h, p.x, p.y)
			for fp := range fresh {
				mainRegion[fp] = true
			}
			break // one flood from any point in the island is enough
		}
		connected++
	}

	fmt.Fprintf(os.Stderr, "Connectivity: connected %d islands, filled %d tiny pockets (%d tiles)\n",
		connected, len(islands)-connected, filled)
}

// carveConnection finds the closest points between two regions and carves
// a walkable corridor between them.
func carveConnection(tiles [][]int, w, h int, mainRegion, island map[point]bool, rng *rand.Rand) {
	// Find the closest pair of points between the two regions.
	// For performance, sample from the island (smaller) and check distance
	// to all main region border points.
	bestDist := math.MaxInt64
	var bestIsland, bestMain point

	// Collect island border points (adjacent to non-walkable)
	var islandBorder []point
	for p := range island {
		for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
			nx, ny := p.x+d[0], p.y+d[1]
			if nx >= 0 && nx < w && ny >= 0 && ny < h && !isWalkable(tiles[ny][nx]) {
				islandBorder = append(islandBorder, p)
				break
			}
		}
	}

	// Collect main region border points
	var mainBorder []point
	for p := range mainRegion {
		for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
			nx, ny := p.x+d[0], p.y+d[1]
			if nx >= 0 && nx < w && ny >= 0 && ny < h && !isWalkable(tiles[ny][nx]) {
				mainBorder = append(mainBorder, p)
				break
			}
		}
	}

	// If borders are huge, sample to keep it fast
	if len(islandBorder) > 200 {
		rng.Shuffle(len(islandBorder), func(i, j int) { islandBorder[i], islandBorder[j] = islandBorder[j], islandBorder[i] })
		islandBorder = islandBorder[:200]
	}
	if len(mainBorder) > 500 {
		rng.Shuffle(len(mainBorder), func(i, j int) { mainBorder[i], mainBorder[j] = mainBorder[j], mainBorder[i] })
		mainBorder = mainBorder[:500]
	}

	for _, ip := range islandBorder {
		for _, mp := range mainBorder {
			d := abs(ip.x-mp.x) + abs(ip.y-mp.y)
			if d < bestDist {
				bestDist = d
				bestIsland = ip
				bestMain = mp
			}
		}
	}

	// Carve a straight-ish corridor between the two points
	x, y := bestIsland.x, bestIsland.y
	tx, ty := bestMain.x, bestMain.y

	for x != tx || y != ty {
		// Move toward target, preferring the longer axis
		if abs(tx-x) >= abs(ty-y) {
			x += sign(tx - x)
		} else {
			y += sign(ty - y)
		}

		if x < 1 || x >= w-1 || y < 1 || y >= h-1 {
			continue
		}

		current := tiles[y][x]
		if isWalkable(current) {
			continue
		}

		// Carve through: water → bridge, everything else → path
		if current == tWater || current == tShallowWater {
			tiles[y][x] = tBridge
		} else {
			tiles[y][x] = tPath
		}
	}
}
