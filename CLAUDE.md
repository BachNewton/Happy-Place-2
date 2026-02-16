# Happy-Place-2

Multiplayer SSH terminal RPG. Players connect via any SSH client, move through a tile-based world rendered with ANSI escape codes, and see each other in real time.

## Tech Stack

- **Language:** Go 1.23
- **SSH:** `gliderlabs/ssh` — username-only auth (username = display name)
- **Maps:** JSON tile maps loaded from `assets/maps/`
- **Renderer:** Double-buffer diff ANSI renderer (only redraws changed cells)

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
    engine.go               # Double-buffer diff renderer + HUD
    combat_view.go          # Combat screen renderer (enemy/player/log/HUD)
    tile_sprites.go         # Tile sprite renderers (15 tile types)
    viewport.go             # Camera coordinate translation
    ansi.go                 # ANSI escape code helpers
  maps/loader.go            # JSON map parser + DefaultMap() fallback
assets/maps/                # JSON tile maps (handcrafted + generated)
docs/procgen.md             # Procedural generation design & reference
```

## Architecture

- **Concurrency:** One goroutine per SSH session + one game loop goroutine at 20 TPS
- **Input:** Session goroutines send `InputEvent` on a shared buffered channel (cap 256)
- **Render:** Game loop broadcasts `GameState` snapshots to per-session render channels (cap 2, non-blocking drops for slow clients)
- **Sync:** `sync.RWMutex` protects player registry; position mutations happen only in the game loop goroutine

## Rendering Terminology

- **Viewport:** The visible portion of the world, defined by camera position (`CamX`/`CamY`) and size (`ViewW`/`ViewH`) in world tile units. Centered on the player, clamped to map edges.
- **World tiles:** The tile grid area filling the upper portion of the screen. Each tile occupies `TileWidth` x `TileHeight` screen cells.
- **HUD:** The bottom 4 rows of the terminal (`HUDRows = 4`): a separator line, then 3 content rows in a two-column layout. Left column: world info, EXP/level, controls. Right column: Health, Stamina, Magic stat bars.
- **Sprite** (`Sprite`): A `TileHeight` x `TileWidth` grid of `SpriteCell`s — the visual representation of a single tile-sized layer. Stamped into the buffer via `stampSprite`.
- **TileSprites** (`TileSprites`): The complete visual output for a tile — a `Base` sprite plus an optional `Overlays` slice. Simple tiles (grass, wall) have only a base. Tall tiles (trees) have a base plus one or more overlays.
- **Overlay** (`Overlay`): A sprite rendered at a vertical offset above its owning tile. Each overlay has a `Sprite` and a `DY` (tile units upward, e.g. `DY=1` means one tile above the base). Overlay cells can be transparent, letting lower layers show through.
- **3-pass rendering:** The render loop in `engine.go` draws in three passes: (1) **Ground pass** stamps `TileSprites.Base` for every visible tile and collects overlays, (2) **Player pass** stamps player sprites, (3) **Overlay pass** stamps collected overlays on top of players. This lets players walk behind tall objects like tree canopies.
- **Tall tile:** A tile whose `TileSprites` includes overlays, making it visually taller than one tile. Trees are 3 tiles tall (base at DY=0, lower canopy at DY=1, upper canopy at DY=2). Built with `tallVariantTile` in `tile_sprites.go`.
- **Buffers (`current` / `next`):** The double-buffer system. `next` is built each frame, diffed against `current`, and only changed cells are emitted as ANSI output.
- **Debug view:** An alternate full-screen view (toggled with `` ` ``) showing all tile sprites and player direction sprites in a grid. Tall tiles display their overlays stacked above the base.

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

### Tile Types

15 tile types have sprite renderers in `tile_sprites.go`:

| Name | Walkable | Notes |
|------|----------|-------|
| grass | yes | 4 variants, animated sway |
| wall | yes | 4 variants, mortar lines |
| water | no | Animated waves, position-aware |
| tree | no | 4 variants, canopy + trunk |
| path | yes | 4 variants, worn center + pebbles |
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
- `@` is the current player; other players show as the first letter of their name
- Player colors rotate through bright ANSI colors (91–96)
- Duplicate usernames get a `_NNNN` suffix appended
