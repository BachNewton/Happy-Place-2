# Happy-Place-2

Multiplayer SSH terminal RPG. Players connect via any SSH client, move through a tile-based world rendered with ANSI escape codes, and see each other in real time.

## Tech Stack

- **Language:** Go 1.23
- **SSH:** `gliderlabs/ssh` — username-only auth (username = display name)
- **Maps:** JSON tile maps loaded from `assets/maps/`
- **Renderer:** Double-buffer diff ANSI renderer (only redraws changed cells)

## Quick Start

```bash
go run ./cmd/server/
# In another terminal:
ssh -o StrictHostKeyChecking=no -p 2222 YourName@localhost
```

## Project Structure

```
cmd/server/main.go          # Entry point, host key gen, wiring
internal/
  server/ssh.go             # SSH server, session handler, input parsing
  game/
    loop.go                 # Game loop (20 TPS), input/broadcast
    player.go               # Player types and input events
    world.go                # World helpers (delegates to maps pkg)
  render/
    engine.go               # Double-buffer diff renderer + HUD
    viewport.go             # Camera coordinate translation
    ansi.go                 # ANSI escape code helpers
  maps/loader.go            # JSON map parser + DefaultMap() fallback
assets/maps/town.json       # Starter map (60x30)
```

## Architecture

- **Concurrency:** One goroutine per SSH session + one game loop goroutine at 20 TPS
- **Input:** Session goroutines send `InputEvent` on a shared buffered channel (cap 256)
- **Render:** Game loop broadcasts `GameState` snapshots to per-session render channels (cap 2, non-blocking drops for slow clients)
- **Sync:** `sync.RWMutex` protects player registry; position mutations happen only in the game loop goroutine

## Controls

- **WASD / Arrow Keys:** Move
- **Q / Ctrl-C:** Quit

## Map Format

Maps are JSON files in `assets/maps/`. See `town.json` for the format. The legend maps tile indices to characters, colors, walkability, and names.

## Development Notes

- Host key is auto-generated as `host_key` in the working directory on first run
- No password auth — username becomes the player's display name
- `@` is the current player; other players show as the first letter of their name
- Player colors rotate through bright ANSI colors (91–96)
- Duplicate usernames get a `_NNNN` suffix appended
