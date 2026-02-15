# Procedural Map Generation

## Overview

The game currently uses handcrafted JSON maps (`assets/maps/`). Procedural generation would add a CLI tool (`cmd/mapgen/`) that outputs maps in the same JSON format. Handcrafted and generated maps coexist — `LoadMaps()` doesn't care how a JSON file was created.

## Workflow

```
go run ./cmd/mapgen/ -type wilderness -seed 42 -size 100x80 -name "Wild Plains" -out assets/maps/wild_plains.json
```

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
- Portal list for map connections

No changes needed to the server, renderer, game loop, or map loader. The generator is a standalone offline tool.

## Map Types

### Wilderness (simplex noise)

The most broadly useful generator. Layers two 2D noise fields to create natural terrain.

**Algorithm:**
- **Elevation noise**: Multiple octaves of simplex noise
- **Moisture noise**: Separate simplex noise field at different frequency

**Tile mapping:**
| Elevation | Moisture | Tile |
|-----------|----------|------|
| Very low | Any | Water |
| Low | High | Flowers / marsh |
| Low | Low | Grass plains |
| Medium | High | Dense forest |
| Medium | Low | Sparse trees + grass |
| High | Any | Mountain walls |

**Why it would look good:** Simplex noise produces organic, flowing shapes — natural-looking rivers, forest edges, and mountain ridges. The existing sprite system already has variants (4 grass types, 4 tree types), animation (grass sway, water ripple), and rich color, so the terrain would feel alive without any extra work.

**Potential new tiles:** Sand (beaches/shores), shallow water (walkable), rock (visual variant of wall for mountains).

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

## Technical Approach

### Simplex Noise

Simplex noise in Go is ~100 lines — no external dependencies needed. It produces smooth, continuous 2D noise perfect for terrain generation.

Key parameters:
- **Frequency**: Controls feature size. Low frequency = large rolling hills, high frequency = small scattered details.
- **Octaves**: Layer multiple noise passes at increasing frequency for natural fractal detail.
- **Lacunarity**: How much frequency increases per octave (typically 2.0).
- **Persistence**: How much amplitude decreases per octave (typically 0.5).

### Map Sizes

Current handcrafted maps are 60x30. Procgen enables much larger maps since there's no manual labor cost:
- **60x30**: Same as current (quick test maps)
- **100x80**: Medium exploration area
- **200x150**: Large wilderness zone
- **300x200**: Epic scale

The viewport and renderer already handle any map size — they only draw visible tiles.

### Spawn Point Selection

The generator needs to pick a valid spawn point:
- Must be on a walkable tile
- Should be in an interesting location (near the center, on a path, in a clearing)
- For wilderness: find a grass clearing near map center
- For caves: spawn in the largest chamber
- For islands: spawn near the center of the landmass

### Portal Placement

Portals are **not** auto-generated. After generating a map, you hand-add portals to connect it to the world:
- Add portal tiles on the new map's edge
- Add matching portal tiles on the connecting map (e.g., Town Square)
- This keeps world topology intentional and hand-designed

## Possible New Tile Types

These would enhance procgen maps but aren't strictly required for a first version:

| Tile | Char | Color | Walkable | Use |
|------|------|-------|----------|-----|
| Sand | `~` | yellow | yes | Beach shores, desert |
| Shallow water | `~` | cyan | yes | Wading areas near shores |
| Rock | `▒` | gray | no | Mountains (visual variant of wall) |
| Tall grass | `;` | bright_green | yes | Meadows, plains |
| Bridge | `=` | yellow | yes | River crossings |
| Dirt | `.` | brown | yes | Transition between grass and path |

## What Makes Procgen Maps Feel Good vs. Bad

**Common pitfalls to avoid:**
- **Uniformity**: Fixed by the variant system — 4 grass variants, 4 tree variants, etc.
- **Noise soup**: Threshold noise into distinct tile types rather than gradients
- **Inaccessible areas**: Flood-fill check that spawn connects to most of the map
- **Boring edges**: Border the map with impassable tiles (trees, walls, water) but don't make it a uniform rectangle — use noise for the boundary shape too
- **No focal points**: Even wilderness needs points of interest — a lake, a clearing, a flower meadow, a mountain peak

**What the existing system gives us for free:**
- Tile variants via `TileHash` → visual variety without any procgen effort
- Animated tiles (grass, water) → maps feel alive
- Connected tiles (fence) → auto-adapt to neighbors
- Rich pixel-art sprites at 10x5 per tile → surprisingly detailed for a terminal game
