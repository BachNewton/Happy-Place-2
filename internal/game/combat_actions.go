package game

import (
	"fmt"
	"math/rand"
)

const (
	MeleeCost  = 5 // stamina
	RangedCost = 2 // stamina
	MagicCost  = 5 // MP
)

// ResolveMelee resolves a melee attack. Returns damage, log message, and whether
// the player had enough stamina.
func ResolveMelee(attacker *Player, target *EnemyInstance) (int, string, bool) {
	if attacker.Stamina < MeleeCost {
		return 0, "", false
	}
	attacker.Stamina -= MeleeCost

	dmg := attacker.Attack + rand.Intn(3) - target.Def.Defense/2
	if dmg < 1 {
		dmg = 1
	}
	target.HP -= dmg
	if target.HP < 0 {
		target.HP = 0
	}

	msg := fmt.Sprintf("%s slashes %s for %d damage!", attacker.Name, target.Label, dmg)
	if !target.Alive() {
		msg += fmt.Sprintf(" %s defeated!", target.Label)
	}
	return dmg, msg, true
}

// ResolveRanged resolves a ranged attack. Weaker but cheaper than melee.
func ResolveRanged(attacker *Player, target *EnemyInstance) (int, string, bool) {
	if attacker.Stamina < RangedCost {
		return 0, "", false
	}
	attacker.Stamina -= RangedCost

	dmg := attacker.Attack/2 + rand.Intn(3) - target.Def.Defense/2
	if dmg < 1 {
		dmg = 1
	}
	target.HP -= dmg
	if target.HP < 0 {
		target.HP = 0
	}

	msg := fmt.Sprintf("%s shoots %s for %d damage!", attacker.Name, target.Label, dmg)
	if !target.Alive() {
		msg += fmt.Sprintf(" %s defeated!", target.Label)
	}
	return dmg, msg, true
}

// ResolveMagic resolves a magic attack. Strongest but costs MP.
func ResolveMagic(attacker *Player, target *EnemyInstance) (int, string, bool) {
	if attacker.MP < MagicCost {
		return 0, "", false
	}
	attacker.MP -= MagicCost

	dmg := attacker.Attack*2 + rand.Intn(4) - target.Def.Defense/3
	if dmg < 1 {
		dmg = 1
	}
	target.HP -= dmg
	if target.HP < 0 {
		target.HP = 0
	}

	msg := fmt.Sprintf("%s casts a spell on %s for %d damage!", attacker.Name, target.Label, dmg)
	if !target.Alive() {
		msg += fmt.Sprintf(" %s defeated!", target.Label)
	}
	return dmg, msg, true
}

// ResolveDefend sets the player to defending stance. Free action.
func ResolveDefend(player *Player) string {
	player.Defending = true
	return fmt.Sprintf("%s braces for impact!", player.Name)
}

// ResolveEnemyAttack resolves an enemy attacking a random living player.
func ResolveEnemyAttack(enemy *EnemyInstance, target *Player) (int, string) {
	dmg := enemy.Def.Attack + rand.Intn(3) - target.Defense/2
	if dmg < 1 {
		dmg = 1
	}
	if target.Defending {
		dmg = dmg / 2
		if dmg < 1 {
			dmg = 1
		}
	}
	target.HP -= dmg
	if target.HP < 0 {
		target.HP = 0
	}

	msg := fmt.Sprintf("%s bites %s for %d damage!", enemy.Label, target.Name, dmg)
	if target.Defending {
		msg += " (Defended!)"
	}
	if target.HP <= 0 {
		target.Dead = true
		msg += fmt.Sprintf(" %s has fallen!", target.Name)
	}
	return dmg, msg
}
