# Happy-Place-2

Multiplayer SSH terminal RPG. Players connect via any SSH client, move through a tile-based world rendered with ANSI escape codes, and see each other in real time.

## Tech Stack

- **Language:** Go 1.23
- **SSH:** `gliderlabs/ssh` — username-only auth (username = display name)
- **Maps:** JSON tile maps loaded from `assets/maps/`
- **Renderer:** Half-block pixel renderer — PNG sprites → pixel buffer → `▄` cells → double-buffer diff ANSI output

## Quick Start

```bash
# Dev server with hot reload (rebuilds + restarts on file changes):
air
# In another terminal:
ssh -o StrictHostKeyChecking=no -p 2222 YourName@localhost
```

## Project Structure

```
cmd/server/main.go          # Entry point, host key gen, wiring
cmd/mapgen/                  # Procedural map generator CLI
  main.go                   #   Wilderness generator + flag parsing
  noise.go                  #   2D simplex noise (self-contained)
cmd/maptools/main.go        # Map validation, viz, stats CLI
internal/
  server/ssh.go             # SSH server, session handler, input parsing
  game/
    loop.go                 # Game loop (20 TPS), input/broadcast, combat integration
    player.go               # Player types and input events
    combat.go               # Fight state machine, CombatState snapshots
    combat_actions.go       # Damage resolution (melee, ranged, magic, defend)
    enemy.go                # Enemy definitions and spawning
    timing.go               # Tick rate and timing constants
    world.go                # World helpers (delegates to maps pkg)
  render/
    engine.go               # Pixel buffer renderer + HUD + debug view
    combat_view.go          # Combat screen renderer (enemy/player/log/HUD)
    pixel.go                # Pixel types (Pixel, PixelSprite, PixelOverlay, PixelTileSprites)
    pixel_tiles.go          # Pixel tile lookup (PixelTileSprite)
    png_loader.go           # PNG sprite loader + SpriteRegistry + player palette swap
    tile_sprites.go         # TileHash, connection masks, tile name ordering
    viewport.go             # PixelViewport camera coordinate translation
    ansi.go                 # ANSI escape code helpers
  maps/loader.go            # JSON map parser + DefaultMap() fallback
assets/maps/                # JSON tile maps (handcrafted + generated)
assets/sprites/tiles/       # 16x16 PNG tile sprites (placeholder art)
assets/sprites/players/     # 16x16 PNG player template sprites
docs/procgen.md             # Procedural generation design & reference
```

## Architecture

- **Concurrency:** One goroutine per SSH session + one game loop goroutine at 20 TPS
- **Input:** Session goroutines send `InputEvent` on a shared buffered channel (cap 256)
- **Render:** Game loop broadcasts `GameState` snapshots to per-session render channels (cap 2, non-blocking drops for slow clients)
- **Sync:** `sync.RWMutex` protects player registry; position mutations happen only in the game loop goroutine

## Rendering Terminology

- **Pixel** (`Pixel`): An `{R, G, B uint8; Transparent bool}` value. The smallest visual unit.
- **PixelSprite** (`PixelSprite`): A `[16][16]Pixel` grid — one tile's visual data. Loaded from 16x16 PNG files.
- **PixelTileSprites** (`PixelTileSprites`): A `Base` PixelSprite plus optional `Overlays` slice. Simple tiles have only a base. Tall tiles (trees) add overlays at DY offsets above the base.
- **PixelOverlay** (`PixelOverlay`): A PixelSprite rendered at a vertical offset above its owning tile. Each has a `Sprite` and a `DY` (tile units upward).
- **Half-block rendering:** Each terminal cell displays 2 vertical pixels using `▄` (U+2584). Background color = top pixel, foreground color = bottom pixel. A 16x16 pixel tile renders as 16 cols x 8 rows of terminal characters.
- **Pixel buffer** (`pixelBuf`): A `[][]Pixel` grid covering the world area (everything above the HUD). Width = terminal columns, height = (termH - HUDRows) * 2 pixels.
- **Tile dimensions:** `PixelTileW = 16`, `PixelTileH = 16` (pixel-space). `CharTileW = 16`, `CharTileH = 8` (screen-space, after half-block collapse).
- **PixelViewport:** Camera coordinates for pixel-based rendering. `CamX`/`CamY` in world tile units, `OffsetX`/`OffsetY` in pixel-buffer coordinates. Centered on player, clamped to map edges.
- **3-pass rendering:** (1) **Ground pass** stamps `PixelTileSprites.Base` into pixelBuf + collects overlays, (2) **Player pass** stamps player PixelSprites, (3) **Overlay pass** stamps collected overlays on top of players. Then `collapsePixelBuf()` converts pixel pairs to half-block `Cell` values in `next`.
- **SpriteRegistry:** Loaded at startup from `assets/sprites/`. Holds all tile and player PixelSprites. Player sprites are palette-swapped from templates (shirt `#FF0000` → player color, pants `#AA0000` → darkened).
- **HUD:** The bottom 4 rows of the terminal (`HUDRows = 4`), rendered character-based directly into `next[][]` after pixel buffer collapse.
- **Buffers (`current` / `next`):** The double-buffer system. `next` is built each frame, diffed against `current`, and only changed cells are emitted as ANSI output.
- **Blob tileset (autotile):** A tile type that adapts its appearance based on 8 surrounding neighbors. Uses 13 source sprites (center, 4 edges, 4 outer corners, 4 inner corners) to generate all 256 possible neighbor configurations at load time. Diagonal neighbors are gated by adjacent cardinals. Used by water; generic system for cliff/path reuse.
- **Debug view:** An alternate full-screen view (toggled with `` ` ``) showing all pixel sprites collapsed via half-block rendering.

### Sprite PNG Naming Conventions

Sprites are loaded from `assets/sprites/tiles/` and `assets/sprites/players/`:
- **Simple:** `grass_0.png` through `grass_3.png` (tile_variant.png)
- **Animated:** `water_0_f0.png`, `water_0_f1.png` (tile_variant_fN.png)
- **Tall:** `tree_0_base.png`, `tree_0_dy1.png`, `tree_0_dy2.png`
- **Connected:** `fence_0_0000.png` through `fence_0_1111.png` (NESW bitmask)
- **Blob (autotile):** `water_0_blob_center.png`, `water_0_blob_edge_n.png`, `water_0_blob_outer_nw.png`, `water_0_blob_inner_nw.png` — 13 named parts (center, 4 edges, 4 outer corners, 4 inner corners). Uses 8-neighbor awareness with diagonal gating. All 256 mask variants are pre-composited at load time.
- **Player:** `player_down.png`, `player_up.png`, `player_left.png`, `player_right.png`

Transparent pixels: alpha < 50% or magenta `#FF00FF`.

## Combat View Terminology

The combat screen (`renderCombatView` in `combat_view.go`) replaces the world view when a player is in a fight. Layout from top to bottom:

- **Enemy area:** `drawEnemyRow()` renders each enemy with a simple ASCII sprite + HP bar. 2 rows per enemy. A `▶` target indicator appears next to the selected target.
- **Battle separator:** A centered `═══ BATTLE  Round N ═══` divider line between enemies and players.
- **Player area:** `drawCombatPlayerRow()` renders each player with a color dot, name, and HP text. 1 row per player. The viewer's row is marked with `←`.
- **Battle log:** The last N combat messages, positioned bottom-up just above the HUD. Most recent message is brighter.
- **Combat HUD:** `drawCombatHUD()`, the bottom `HUDRows` (4) rows with a two-column layout. Left column: turn info, action selection (1-4), target/confirm hints. Right column: Health, Stamina, Magic stat bars. Selected action is highlighted bright/bold.
- **Transition flash:** A red/black pulsing screen with centered `!! ENCOUNTER !!` text, shown for ~1 second before combat starts.
- **Victory/Defeat overlay:** Centered `★ VICTORY ★` or `✖ DEFEAT ✖` text drawn over the middle of the screen during result phase.

## Game World Terminology

- **Map** (`maps.Map`): A loaded tile map — the data layer containing tiles, legend, dimensions, name, and spawn point. Loaded from JSON files in `assets/maps/`. Each map has a `Name` (e.g. `"Town"`) displayed in the HUD.
- **World** (`game.World`): The gameplay-level wrapper around a Map. Provides game logic helpers like `CanMoveTo()` and `SpawnPoint()`. Currently holds a single Map, but designed as the natural place to expand to multiple maps.
- **GameState**: The per-tick snapshot broadcast to each session for rendering. Contains all `PlayerSnapshot`s, a `Map` pointer, and the tick count. Currently world-scoped (one map, all players); would become map-scoped when multiple maps are added (each session receives only its map and co-located players).

## Controls

### Overworld
- **WASD / Arrow Keys:** Move (first press turns, second press moves)
- **`` ` `` (backtick):** Toggle debug sprite view
- **`~` (tilde):** Force-start combat encounter (debug)
- **Q / Ctrl-C:** Quit

### Combat
- **1 / 2 / 3:** Select action (Melee / Ranged / Magic)
- **4:** Defend (immediate, no target needed)
- **← →:** Cycle enemy target
- **Enter:** Confirm action on selected target

## Map Format

Maps are JSON files in `assets/maps/`. See `town.json` for the format. The legend maps tile indices to characters, colors, walkability, and names.

**Building orientation:** Buildings must face south (door on the south wall). The 3-pass rendering system draws overlays on top of players, so tall tiles visually extend upward. Players approaching from the south walk behind overlays (e.g. tree canopies, future roofs). A north-facing door breaks this — the player would be above the building with no layering benefit.

**Blob tile spacing:** Blob/border-blob tiles (water, path) use a 13-piece tileset that only handles contiguous regions. A non-blob tile (e.g. grass) with blob neighbors on **opposite sides** (N+S or E+W) has no valid sprite — the transition breaks visually. To avoid this: blob tile bodies must be at least 2 tiles wide, and gaps between separate blob bodies must be at least 2 tiles. Never leave a single-tile gap of grass between two path/water regions. Run `go run _tmp/main.go` with the audit script (or visually check with the `T` tile overlay) after editing maps.

### Tile Types

15 tile types have PNG sprites in `assets/sprites/tiles/`:

| Name | Walkable | Notes |
|------|----------|-------|
| grass | yes | 4 variants, animated sway |
| wall | yes | 4 variants, mortar lines |
| water | no | Blob autotile (8-neighbor edges/corners) |
| tree | no | 4 variants, canopy + trunk |
| path | yes | Blob autotile (8-neighbor edges/corners) |
| door | yes | Plank lines + doorknob |
| floor | yes | 4 variants, wood grain |
| fence | no | Connected tile (adapts to neighbors) |
| flowers | yes | 6 color variants, stems |
| sand | yes | 4 variants, animated grain dots |
| tall_grass | yes | 4 variants, animated dense blades |
| rock | no | 4 variants, craggy texture |
| shallow_water | yes | Animated, lighter than deep water |
| dirt | yes | 4 variants, pebble dots |
| bridge | yes | 2 variants, planks + rails |

## Map Generation (experimental, not production-ready)

`cmd/mapgen/` exists but generates low-quality maps. **Do not use it** — all maps should be handcrafted. See `docs/procgen.md` for design notes if the generator is revisited later.

## Production Deployment

The game server runs on an Oracle Cloud always-free VM (VM.Standard.E2.1.Micro, 1 OCPU, 1GB RAM, Ubuntu 24.04).

**Connect to the game:**
```bash
ssh -p 2222 YourName@207.127.95.242
```

**SSH into the VM:**
```bash
ssh -i ~/.ssh/oci_happy_place ubuntu@207.127.95.242
```

**Auto-deploy:** Every push to `main` triggers `.github/workflows/deploy.yml`, which SSHes into the VM, pulls the latest code, rebuilds, and restarts the service.

**Service management (on the VM):**
```bash
sudo systemctl status happy-place    # Check status
sudo systemctl restart happy-place   # Restart
sudo journalctl -u happy-place -f    # View logs
```

**Infrastructure details:**
- **Region:** eu-stockholm-1
- **OCI CLI config:** `~/.oci/config`
- **VM SSH key:** `~/.ssh/oci_happy_place`
- **GitHub secrets:** `OCI_SSH_KEY`, `OCI_HOST`
- **Systemd unit:** `/etc/systemd/system/happy-place.service`
- **App path on VM:** `/home/ubuntu/Happy-Place-2`

## Development Notes

- Host key is auto-generated as `host_key` in the working directory on first run
- No password auth — username becomes the player's display name
- Player sprites are 16x16 PNGs with palette-swapped shirt/pants colors
- Player colors rotate through 6 RGB shirt colors (defined in `PlayerBGColors`)
- Duplicate usernames get a `_NNNN` suffix appended
