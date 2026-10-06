package domain

import "testing"

func TestEnemyCannotMoveOutsideMap(t *testing.T) {
	g := NewGame(42)
	g.Level.Enemies = []Enemy{{Kind: EnemySnake, Health: 10, Pos: Point{X: 0, Y: g.Level.Height - 1}, Hostility: 0}}
	g.Player.Pos = Point{X: 20, Y: 10}
	g.enemyTurns()
	if !g.inBounds(g.Level.Enemies[0].Pos) {
		t.Fatal("enemy left the map")
	}
}

func TestGenerateLevelHasNineConnectedRooms(t *testing.T) {
	g := NewGame(42)
	level := g.Level
	if len(level.Rooms) != 9 {
		t.Fatalf("rooms = %d, want 9", len(level.Rooms))
	}
	for _, room := range level.Rooms {
		if !g.pathExists(level.Rooms[level.StartRoom].Center, room.Center) {
			t.Fatalf("room %d is not reachable from start room", room.ID)
		}
	}
	if level.Tiles[level.Rooms[level.ExitRoom].Center.Y][level.Rooms[level.ExitRoom].Center.X] != TileExit {
		t.Fatalf("exit room center is not an exit tile")
	}
}

func TestBackpackCapsNonTreasureItemsAtNine(t *testing.T) {
	bag := NewBackpack()
	for i := 0; i < 9; i++ {
		if !bag.Add(Item{Kind: ItemFood, Health: 1}) {
			t.Fatalf("item %d should fit", i)
		}
	}
	if bag.Add(Item{Kind: ItemFood, Health: 1}) {
		t.Fatalf("tenth food item should not fit")
	}
}

func TestUseFoodAndWeapon(t *testing.T) {
	g := NewGame(7)
	g.Player.Health = 10
	g.Player.Bag.Add(Item{Kind: ItemFood, Health: 5})
	g.UseItem(ItemFood, 0)
	if g.Player.Health != 15 {
		t.Fatalf("health = %d, want 15", g.Player.Health)
	}
	g.Player.Bag.Add(Item{Kind: ItemWeapon, SubType: "test", Strength: 4})
	g.UseItem(ItemWeapon, 0)
	if g.Player.Weapon == nil || g.Player.Weapon.Strength != 4 {
		t.Fatalf("weapon was not equipped")
	}
	g.UseItem(ItemWeapon, -1)
	if g.Player.Weapon != nil {
		t.Fatalf("weapon was not unequipped")
	}
	if got := len(g.Player.Bag.Items[ItemWeapon]); got != 1 {
		t.Fatalf("weapons in bag = %d, want 1", got)
	}
}

func TestCombatDefeatsAdjacentEnemyAndAwardsTreasure(t *testing.T) {
	g := NewGame(99)
	g.Level.Enemies = []Enemy{{
		ID:        1,
		Kind:      EnemyZombie,
		Health:    1,
		MaxHealth: 1,
		Agility:   1,
		Strength:  1,
		Hostility: 1,
		Pos:       Point{X: g.Player.Pos.X + 1, Y: g.Player.Pos.Y},
	}}
	g.Player.Strength = 50
	g.Player.Agility = 50
	g.Move(1, 0)
	if len(g.Level.Enemies) != 0 {
		t.Fatalf("enemy should be defeated")
	}
	if g.Stats.Defeated != 1 {
		t.Fatalf("defeated = %d, want 1", g.Stats.Defeated)
	}
	if g.Stats.Treasure <= 0 {
		t.Fatalf("expected treasure reward")
	}
}

func TestDoorsKeysAndSolvability(t *testing.T) {
	level := GenerateLevelWithBalance(12, 1234, 0)
	if len(level.Doors) == 0 {
		t.Fatalf("expected locked doors on deeper level")
	}
	keys := map[DoorColor]bool{}
	for _, item := range level.Items {
		if item.Item.Kind == ItemKey {
			keys[DoorColor(item.Item.SubType)] = true
		}
	}
	for _, door := range level.Doors {
		if !keys[door.Color] {
			t.Fatalf("door %s has no key", door.Color)
		}
	}
	if !keysAndDoorsAreSolvable(level) {
		t.Fatalf("generated doors and keys should be solvable")
	}
}

func TestLockedDoorRequiresKey(t *testing.T) {
	g := NewGame(55)
	g.Level = GenerateLevelWithBalance(12, 55, 0)
	if len(g.Level.Doors) == 0 {
		t.Fatalf("expected at least one door")
	}
	door := g.Level.Doors[0]
	g.Player.Bag.Keys = map[DoorColor]bool{}
	if g.pathWalkable(door.Pos) {
		t.Fatalf("locked door should block without key")
	}
	g.Player.Bag.Keys[door.Color] = true
	if !g.pathWalkable(door.Pos) {
		t.Fatalf("door should become passable with key")
	}
}

func TestAdaptiveBalanceChangesOnDescent(t *testing.T) {
	g := NewGame(77)
	g.Player.Health = g.Player.MaxHealth
	g.Stats.HitsTaken = 0
	g.adjustBalance()
	if g.Balance <= 0 {
		t.Fatalf("balance = %d, want harder balance after easy level", g.Balance)
	}
	g.Player.Health = 1
	g.adjustBalance()
	if g.Balance >= 1 {
		t.Fatalf("balance = %d, want reduced balance after hard level", g.Balance)
	}
}
