package game

import "happy-place-2/internal/maps"

// World wraps multiple Maps and provides game-level helpers.
type World struct {
	Maps       map[string]*maps.Map
	DefaultMap string
}

// NewWorld creates a world from the given map registry.
func NewWorld(allMaps map[string]*maps.Map, defaultMap string) *World {
	return &World{Maps: allMaps, DefaultMap: defaultMap}
}

// SpawnPoint returns the default map's name and spawn coordinates.
func (w *World) SpawnPoint() (string, int, int) {
	m := w.Maps[w.DefaultMap]
	return w.DefaultMap, m.SpawnX, m.SpawnY
}

// CanMoveTo checks if the destination tile is walkable on the named map.
func (w *World) CanMoveTo(mapName string, x, y int) bool {
	m, ok := w.Maps[mapName]
	if !ok {
		return false
	}
	return m.IsWalkable(x, y)
}

// PortalAt returns the portal at the given position on the named map, or nil.
func (w *World) PortalAt(mapName string, x, y int) *maps.Portal {
	m, ok := w.Maps[mapName]
	if !ok {
		return nil
	}
	return m.PortalAt(x, y)
}

// InteractionAt returns the interaction at the given position on the named map, or nil.
func (w *World) InteractionAt(mapName string, x, y int) *maps.Interaction {
	m, ok := w.Maps[mapName]
	if !ok {
		return nil
	}
	return m.InteractionAt(x, y)
}

// GetMap returns the map with the given name, or nil.
func (w *World) GetMap(name string) *maps.Map {
	return w.Maps[name]
}
