# Procedural Map Generation

## Overview

The game uses handcrafted JSON maps (`assets/maps/`) alongside procedurally generated ones. The CLI tool `cmd/mapgen/` outputs maps in the same JSON format — `LoadMaps()` doesn't care how a JSON file was created.

## Usage

```bash
go run ./cmd/mapgen/ -type wilderness -seed 42 -size 100x80 -name "Wild Plains" -out assets/maps/wild_plains.json
```

**Flags:**
- `-type` — generator type (required; currently: `wilderness`)
- `-seed` — random seed, 0 = random (default `0`)
- `-size` — map dimensions as `WxH` (default `100x80`, minimum `10x10`)
- `-name` — map display name (default `Wilderness`)
- `-out` — output file path (default: stdout)

## Workflow

1. Run the generator with a seed, type, size, and name
2. Validate with `go run ./cmd/maptools/ validate assets/maps/`
3. Preview with `go run ./cmd/maptools/ viz assets/maps/wild_plains.json`
4. Hand-edit if desired — add portals, buildings, NPC spawns, quest items
5. Commit the final JSON to the repo

The seed is deterministic — same seed always produces the same map. Record it in the filename or a comment so you can regenerate from scratch.

## Compatibility

Generated maps use the exact same JSON format as handcrafted maps:
- `[][]int` tile grid with legend
- Spawn point
- Portal list (empty by default — add by hand)

No changes needed to the server, renderer, game loop, or map loader. The generator is a standalone offline tool.

## Implemented: Wilderness

The wilderness generator layers three simplex noise fields to create natural terrain with 12 tile types.

**Noise fields:**
- **Elevation** — freq 0.02, 4 octaves (landforms)
- **Moisture** — freq 0.03, 3 octaves (biome selection)
- **Detail** — freq 0.1, 2 octaves (sparse/dense variation)

**Tile mapping by elevation bands:**

| Elevation | Condition | Tile |
|-----------|-----------|------|
| < 0.20 | — | water |
| 0.20–0.28 | — | shallow_water |
| 0.28–0.32 | — | sand |
| 0.32–0.42 | moisture > 0.6 | flowers |
| 0.32–0.42 | moisture > 0.45 | tall_grass |
| 0.32–0.42 | else | grass |
| 0.42–0.70 | moisture > 0.55 | tree (dense forest) |
| 0.42–0.70 | moisture > 0.35 | tree/tall_grass/grass mix via detail noise |
| 0.42–0.70 | else | grass |
| 0.70–0.78 | — | rock |
| >= 0.78 | — | wall (mountain peaks) |

**Post-processing:**
- **Trail carving**: Random walk from spawn toward 2–3 edge points. Places path tiles along the walk, dirt alongside on grass/tall_grass, bridge where crossing water/shallow_water.
- **Edge treatment**: Outermost ring forced impassable (trees or wall by elevation). Border zone (3 tiles deep) uses noise-shaped boundary converting walkable tiles to trees/rock.
- **Spawn selection**: Spirals outward from map center, finds grass/path tile with mostly-walkable 3×3 neighborhood (7+ of 9 tiles walkable).

**Typical distribution** (100x80, seed 42): ~33% grass+tall_grass, ~33% tree, ~13% water+shallow_water, ~9% rock+wall, ~5% sand, ~5% flowers, ~2% path/dirt/bridge.

**Legend — 12 tile types:**

| Idx | Name | Char | Color | Walk | Notes |
|-----|------|------|-------|------|-------|
| 0 | grass | `.` | green | yes | Matches existing |
| 1 | water | `~` | blue | no | Matches existing |
| 2 | tree | `T` | green | no | Matches existing |
| 3 | wall | `#` | gray | no | Matches existing |
| 4 | flowers | `*` | bright_red | yes | Matches existing |
| 5 | path | `.` | yellow | yes | Matches existing |
| 6 | sand | `~` | yellow | yes | New — beach shores |
| 7 | tall_grass | `;` | bright_green | yes | New — meadows |
| 8 | rock | `▒` | gray | no | New — mountain outcrops |
| 9 | shallow_water | `~` | cyan | yes | New — wading areas |
| 10 | dirt | `.` | yellow | yes | New — trail borders |
| 11 | bridge | `=` | yellow | yes | New — river crossings |

## Future Map Types

### Caves (cellular automata)

Classic roguelike cave generation.

**Algorithm:**
1. Fill grid randomly: ~45% wall, ~55% floor
2. Iterate 4-5 times: a cell becomes wall if 5+ of its 8 neighbors are walls
3. Flood-fill to find disconnected regions
4. Carve tunnels between disconnected chambers using path tiles
5. Add water pools in low spots, flowers as underground mushrooms

**Why it would look good:** Cellular automata produces natural-looking cave systems with irregular chambers and winding passages. Wall sprites with mortar lines would read as carved stone. Floor sprites as cave floor.

### Islands (radial gradient + noise)

**Algorithm:**
- Compute distance from center of map → base elevation (high center, low edges)
- Add simplex noise to break the circle into natural coastline
- Threshold: water at edges, grass/trees/flowers inland
- Optional: add a sand tile type for beach transition

**Why it would look good:** Water sprites surround the island with animated waves. The irregular coastline from noise prevents the "obvious circle" problem. Forests clustered inland, meadows and flowers near the coast.

### River Valleys (noise + erosion)

**Algorithm:**
- Generate elevation noise
- Pick entry/exit points on opposite map edges
- Carve a river using a random walk biased downhill
- Widen the river channel (2-4 tiles)
- Place path tiles along one bank as a trail
- Scatter trees and flowers on the terrain

**Why it would look good:** A river cutting through a landscape is inherently scenic. The animated water sprites would draw the eye. Path along the bank gives players a natural route to follow.

### Villages (BSP + noise)

**Algorithm:**
- Generate background terrain with noise (grass, scattered trees, flowers)
- Use BSP (binary space partitioning) to divide the map into zones
- Place rectangular buildings in some zones: wall perimeter, floor interior, door on one side
- Connect building entrances with A* pathfinding → path tiles
- Optionally add fence-enclosed gardens with flowers

**Why it would look good:** Buildings would use connected wall sprites with proper mortar patterns. Doors, floors, fences — all existing tile types. Paths winding between buildings through grassy terrain with trees gives a natural village feel.

## Technical Details

### Simplex Noise (`cmd/mapgen/noise.go`)

Self-contained 2D simplex noise implementation (~110 lines, only imports `math` and `math/rand`):
- `SimplexNoise` struct with seed-shuffled permutation table
- `Noise2D(x, y)` — core 2D simplex noise, returns [-1, 1]
- `Fractal(x, y, freq, octaves, lacunarity, persistence)` — multi-octave noise normalized to [0, 1]

Key parameters:
- **Frequency**: Controls feature size. Low frequency = large rolling hills, high frequency = small scattered details.
- **Octaves**: Layer multiple noise passes at increasing frequency for natural fractal detail.
- **Lacunarity**: How much frequency increases per octave (2.0).
- **Persistence**: How much amplitude decreases per octave (0.5).

### Map Sizes

Handcrafted maps are 60x30. Procgen enables much larger maps:
- **60x30**: Quick test maps
- **100x80**: Medium exploration area (default)
- **200x150**: Large wilderness zone
- **300x200**: Epic scale

The viewport and renderer already handle any map size — they only draw visible tiles.

### Portal Placement

Portals are **not** auto-generated. After generating a map, hand-add portals to connect it to the world:
- Add portal entries to the generated JSON
- Add matching portal tiles on the connecting map (e.g., Town Square)
- This keeps world topology intentional and hand-designed

## Design Notes

**What the existing system gives us for free:**
- Tile variants via `TileHash` → visual variety without any procgen effort
- Animated tiles (grass, water, sand, tall_grass, shallow_water) → maps feel alive
- Connected tiles (fence) → auto-adapt to neighbors
- Rich pixel-art sprites at 10x5 per tile → surprisingly detailed for a terminal game

**Pitfalls avoided:**
- **Uniformity**: Fixed by the variant system — 4 grass variants, 4 tree variants, etc.
- **Noise soup**: Noise is thresholded into distinct tile types, not used as gradients
- **Boring edges**: Border uses noise-shaped boundary, not a uniform rectangle
- **No focal points**: Elevation bands create natural features — lakes, clearings, flower meadows, mountain ridges
