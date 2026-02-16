# Future Tall Tiles

Ideas for tiles that use the overlay system to extend visually above their base. Each entry describes the tile height, what the layers look like, and where it would appear in-game.

## Roofs (2 tiles)

Buildings currently have flat wall tops. A roof tile would replace the top wall row of each building.

- **Base (DY=0):** Eaves — horizontal overhang extending past the building walls. Dark wood or slate colored.
- **Overlay (DY=1):** Peaked roof — triangular ridge tapering to a point. Shingle texture (`≡` or `▓`). Transparent cells at the sloped edges so trees and sky show through.

Gives buildings a classic RPG look and lets players walk behind the roofline from the south.

## Lamp Posts (2 tiles)

Decorative street lighting along paths and in plazas.

- **Base (DY=0):** Dimmed grass/path with a thin pole (single column, `│`) and a stone base.
- **Overlay (DY=1):** Lantern head — a small bright yellow/orange glow (`◆` or `■`) with transparent surrounds. Could animate with a flicker (2-3 frame cycle alternating warm tones).

Non-walkable. Place at path intersections and building entrances.

## Archways (2 tiles)

Walkable passage through a wall — the player walks under the arch and the top renders over them.

- **Base (DY=0):** Walkable path/floor tile with stone pillar columns on left and right edges.
- **Overlay (DY=1):** Stone arch (`╔═══╗` style) spanning the full width. Transparent center so the ground and players show through the opening.

Useful for town gates, garden entrances, and building courtyards.

## Tall Mushrooms (2 tiles)

Giant fantasy mushrooms for forests and swamps.

- **Base (DY=0):** Dimmed grass with a thick stem (2-3 columns, pale white/tan).
- **Overlay (DY=1):** Wide spotted cap — dome shape using `●` or `◠` characters. Red with white spots, or blue/purple for variety. Transparent edges.

Non-walkable. Mix with trees in forest maps for a fairy-tale atmosphere.

## Windmill (3 tiles)

Landmark building for farm or village areas.

- **Base (DY=0):** Stone base building with a door. Non-walkable.
- **Overlay (DY=1):** Upper tower — narrower stone section tapering inward.
- **Overlay (DY=2):** Sails — animated rotating blades (`/`, `─`, `\`, `|` cycle). Transparent background so sky/terrain shows through. 4-frame animation.

One per map as a landmark. Visible from far away due to height.

## Watchtower (3 tiles)

Defensive structure at map edges or town borders.

- **Base (DY=0):** Stone foundation with an open doorway (walkable entrance). Thick walls on left/right.
- **Overlay (DY=1):** Narrow tower shaft — stone walls with small window slits (`▪`).
- **Overlay (DY=2):** Crenellated top — battlements (`┬┬┬`) with a flag or torch. Transparent gaps between merlons.

Non-walkable except the doorway. Place at roads entering town.

## Cliff Face (2 tiles)

Vertical terrain for elevation changes at map edges.

- **Base (DY=0):** Rocky cliff wall — dark grey/brown stone texture (`▒`, `░`). Non-walkable.
- **Overlay (DY=1):** Cliff top — grass or dirt ledge at the top with transparent lower portion showing the rock face. Could have sparse vegetation clinging to the edge.

Use as a natural map boundary instead of plain walls. Line the north edge of a map to imply higher ground beyond.

## Statue / Monument (2 tiles)

Town square centerpiece or memorial.

- **Base (DY=0):** Stone pedestal on path — rectangular base (`▐█▌`), walkable around edges.
- **Overlay (DY=1):** Figure or obelisk — sword-wielding hero, angel wings, or simple pointed spire. Transparent background.

Non-walkable. Place at path intersections as landmarks for navigation.

## Totem Pole (3 tiles)

Decorative tribal/mystical structure for wilderness or special areas.

- **Base (DY=0):** Dimmed grass with carved wooden base — wide bottom face with eyes/mouth (`◉`).
- **Overlay (DY=1):** Middle face — different expression, wings or arms extending sideways.
- **Overlay (DY=2):** Top face — smallest, with feathers or horns at the peak. Transparent edges.

Non-walkable. 2-3 variants with different face combinations.

## Waterfall (3 tiles)

Animated water feature cascading down a cliff.

- **Base (DY=0):** Pool at the bottom — animated water tile (`~`) with splash foam (`░`) and mist. Non-walkable.
- **Overlay (DY=1):** Falling water — animated vertical streams (`│`, `┃`, `║` cycling) in blue/white. Rocky walls on sides. Transparent gaps for mist effect.
- **Overlay (DY=2):** Cliff top with water source — river/stream flowing to the edge. Rock on sides, transparent center showing water.

Animated across all 3 layers. Place against cliff faces or at map edges.

## Hanging Vines / Willow (2 tiles)

Overlay-only addition to existing trees or archways.

- **Base (DY=0):** Normal tree or arch base (unchanged).
- **Overlay (DY=1):** Drooping vine strands (`\`, `|`, `/`) in green, hanging down from canopy. Mostly transparent with scattered vine characters. Could animate with a gentle sway (2-frame cycle shifting vine positions by 1 column).

Adds atmosphere to forest and swamp maps without a new tile type — could be a tree variant.

## Campfire (1 tile + animated overlay)

Small rest point along roads.

- **Base (DY=0):** Dimmed grass with a stone ring (`○`) and log seats. Non-walkable.
- **Overlay (DY=1, partial height):** Flame and smoke — animated fire (`▲`, `♦` in orange/red cycling) rising above the pit. Smoke particles (`·`, `°`) drifting upward in grey. Transparent background.

Walkable around it. Gathering point for players. Could be an interaction point ("Rest by the campfire").
