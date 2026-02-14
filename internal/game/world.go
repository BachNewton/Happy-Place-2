package game

import "happy-place-2/internal/maps"

// World wraps a Map and provides game-level helpers.
type World struct {
	Map *maps.Map
}

// NewWorld creates a world from the given map.
func NewWorld(m *maps.Map) *World {
	return &World{Map: m}
}

// SpawnPoint returns the map's spawn coordinates.
func (w *World) SpawnPoint() (int, int) {
	return w.Map.SpawnX, w.Map.SpawnY
}

// CanMoveTo checks if the destination tile is walkable.
func (w *World) CanMoveTo(x, y int) bool {
	return w.Map.IsWalkable(x, y)
}
