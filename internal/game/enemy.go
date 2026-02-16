package game

// EnemyDef defines an enemy type's base stats.
type EnemyDef struct {
	Name    string
	MaxHP   int
	Attack  int
	Defense int
	EXP     int // awarded per kill
}

// EnemyInstance is a live enemy in a fight.
type EnemyInstance struct {
	Def   EnemyDef
	HP    int
	ID    int    // unique within the fight (0-based)
	Label string // display name, e.g. "Rat A"
}

// Alive reports whether this enemy still has HP.
func (e *EnemyInstance) Alive() bool {
	return e.HP > 0
}

// EnemyRat is the basic encounter enemy.
var EnemyRat = EnemyDef{
	Name:    "Rat",
	MaxHP:   15,
	Attack:  4,
	Defense: 1,
	EXP:     8,
}

// enemyLabels generates labels like "Rat A", "Rat B", ... for N enemies.
func enemyLabels(count int, name string) []string {
	labels := make([]string, count)
	for i := 0; i < count; i++ {
		if count == 1 {
			labels[i] = name
		} else {
			labels[i] = name + " " + string(rune('A'+i))
		}
	}
	return labels
}

// spawnEnemies creates N instances of a given EnemyDef.
func spawnEnemies(def EnemyDef, count int) []*EnemyInstance {
	labels := enemyLabels(count, def.Name)
	enemies := make([]*EnemyInstance, count)
	for i := 0; i < count; i++ {
		enemies[i] = &EnemyInstance{
			Def:   def,
			HP:    def.MaxHP,
			ID:    i,
			Label: labels[i],
		}
	}
	return enemies
}
