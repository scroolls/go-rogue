package domain

import (
	"fmt"
	"math"
	"math/rand"
)

func NewGame(seed int64) *Game {
	g := &Game{}
	g.NewRun(seed)
	return g
}

func LoadFromSnapshot(state SaveState) *Game {
	g := state.Game
	if g.Player.Bag.Items == nil {
		g.Player.Bag = NewBackpack()
	}
	if g.Player.Bag.Keys == nil {
		g.Player.Bag.Keys = map[DoorColor]bool{}
	}
	if g.State == "" {
		g.State = StatePlaying
	}
	return &g
}

func (g *Game) NewRun(seed int64) {
	g.Seed = seed
	g.Turn = 0
	g.State = StatePlaying
	g.Stats = RunStats{ReachedLevel: 1}
	g.SleepTurns = 0
	g.Balance = 0
	g.Level = GenerateLevelWithBalance(1, seed, g.Balance)
	start := g.startPoint()
	g.Player = Character{
		MaxHealth: 35,
		Health:    35,
		Agility:   9,
		Strength:  6,
		Pos:       start,
		Bag:       NewBackpack(),
	}
	g.Message = "Новая экспедиция началась."
	g.revealAroundPlayer()
}

func (g *Game) Snapshot() SaveState {
	return SaveState{Game: *g}
}

func (g *Game) Move(dx, dy int) EventResult {
	if g.State != StatePlaying {
		return EventResult{Message: g.Message, Done: true}
	}
	if g.SleepTurns > 0 {
		g.SleepTurns--
		g.Message = "Вы спите и пропускаете ход."
		g.endTurn()
		return EventResult{Message: g.Message}
	}
	target := Point{X: g.Player.Pos.X + dx, Y: g.Player.Pos.Y + dy}
	if !g.inBounds(target) || !g.isWalkable(target) {
		g.Message = "Путь закрыт."
		return EventResult{Message: g.Message}
	}
	if idx := g.enemyAt(target); idx >= 0 {
		g.playerAttack(idx)
		g.endTurn()
		return EventResult{Message: g.Message, Done: g.State != StatePlaying}
	}
	g.Player.Pos = target
	g.Stats.CellsWalked++
	g.pickupAt(target)
	if g.tileAt(target) == TileExit {
		g.descend()
	} else {
		g.endTurn()
	}
	g.revealAroundPlayer()
	return EventResult{Message: g.Message, Done: g.State != StatePlaying}
}

func (g *Game) UseItem(kind ItemKind, index int) EventResult {
	if g.State != StatePlaying {
		return EventResult{Message: g.Message, Done: true}
	}
	if kind == ItemWeapon && index == -1 {
		if g.Player.Weapon != nil {
			if !g.Player.Bag.Add(*g.Player.Weapon) {
				g.Message = "В рюкзаке нет места для снятого оружия."
				return EventResult{Message: g.Message}
			}
			g.Player.Weapon = nil
			g.Message = "Оружие убрано в рюкзак."
			g.endTurn()
		}
		return EventResult{Message: g.Message}
	}
	item, ok := g.Player.Bag.Remove(kind, index)
	if !ok {
		g.Message = "Нет такого предмета."
		return EventResult{Message: g.Message}
	}
	switch item.Kind {
	case ItemFood:
		g.Player.Health += item.Health
		if g.Player.Health > g.Player.MaxHealth {
			g.Player.Health = g.Player.MaxHealth
		}
		g.Stats.FoodEaten++
		g.Message = fmt.Sprintf("Еда восстановила %d здоровья.", item.Health)
	case ItemElixir:
		g.applyItemStats(item)
		g.Player.Effects = append(g.Player.Effects, itemEffects(item, max(1, item.Duration))...)
		g.Stats.ElixirsDrunk++
		g.Message = "Эликсир временно усилил характеристики."
	case ItemScroll:
		g.applyItemStats(item)
		g.Stats.ScrollsRead++
		g.Message = "Свиток навсегда усилил характеристики."
	case ItemWeapon:
		if g.Player.Weapon != nil {
			g.dropNearPlayer(*g.Player.Weapon)
		}
		g.Player.Weapon = &item
		g.Message = fmt.Sprintf("В руках оружие силы %d.", item.Strength)
	}
	g.endTurn()
	return EventResult{Message: g.Message, Done: g.State != StatePlaying}
}

func (g *Game) Inventory(kind ItemKind) []Item {
	return append([]Item(nil), g.Player.Bag.Items[kind]...)
}

func (g *Game) CurrentRecord() RunRecord {
	return RunRecord{Stats: g.Stats, Won: g.State == StateWon}
}

func (g *Game) playerAttack(idx int) {
	enemy := &g.Level.Enemies[idx]
	if enemy.Kind == EnemyVampire && enemy.FirstMiss {
		enemy.FirstMiss = false
		g.Message = "Первый удар по вампиру прошел мимо."
		return
	}
	if !hit(g.Player.Agility, enemy.Agility, g.rng()) {
		g.Message = "Вы промахнулись."
		return
	}
	damage := g.Player.Strength + g.rng().Intn(4)
	if g.Player.Weapon != nil {
		damage += g.Player.Weapon.Strength + g.rng().Intn(max(1, g.Player.Weapon.Strength))
	}
	enemy.Health -= damage
	g.Stats.HitsDealt++
	if enemy.Health <= 0 {
		treasure := enemyTreasure(*enemy, g.rng())
		g.Stats.Treasure += treasure
		g.Stats.Defeated++
		g.Level.Enemies = append(g.Level.Enemies[:idx], g.Level.Enemies[idx+1:]...)
		g.Message = fmt.Sprintf("Враг побежден, найдено сокровищ: %d.", treasure)
		return
	}
	g.Message = fmt.Sprintf("Вы нанесли %d урона.", damage)
}

func (g *Game) endTurn() {
	if g.State != StatePlaying {
		return
	}
	g.Turn++
	g.tickEffects()
	g.enemyTurns()
	if g.Player.Health <= 0 {
		g.Player.Health = 0
		g.State = StateDead
		g.Message = "Герой погиб. Результат записан в таблицу."
	}
	g.revealAroundPlayer()
}

func (g *Game) enemyTurns() {
	for i := range g.Level.Enemies {
		enemy := &g.Level.Enemies[i]
		if enemy.Health <= 0 {
			continue
		}
		if adjacent(enemy.Pos, g.Player.Pos) {
			g.enemyAttack(enemy)
			continue
		}
		next := g.enemyNextPoint(enemy)
		if next == enemy.Pos || !g.inBounds(next) || !g.isWalkable(next) || g.enemyAt(next) >= 0 || next == g.Player.Pos {
			continue
		}
		enemy.Pos = next
		if adjacent(enemy.Pos, g.Player.Pos) {
			g.enemyAttack(enemy)
		}
	}
}

func (g *Game) enemyAttack(enemy *Enemy) {
	if enemy.Kind == EnemyOgre && enemy.Resting {
		enemy.Resting = false
		return
	}
	if hit(enemy.Agility, g.Player.Agility, g.rng()) {
		damage := enemy.Strength + g.rng().Intn(4)
		g.Player.Health -= damage
		g.Stats.HitsTaken++
		g.Message = fmt.Sprintf("%s попал по вам на %d.", enemyLabel(enemy.Kind), damage)
		if enemy.Kind == EnemyVampire && g.Player.MaxHealth > 5 {
			g.Player.MaxHealth--
			if g.Player.Health > g.Player.MaxHealth {
				g.Player.Health = g.Player.MaxHealth
			}
		}
		if enemy.Kind == EnemySnake && g.rng().Intn(100) < 25 {
			g.SleepTurns = 1
		}
	} else {
		g.Message = fmt.Sprintf("%s промахнулся.", enemyLabel(enemy.Kind))
	}
	if enemy.Kind == EnemyOgre {
		enemy.Resting = true
	}
}

func (g *Game) enemyNextPoint(enemy *Enemy) Point {
	if distance(enemy.Pos, g.Player.Pos) <= float64(enemy.Hostility) && g.pathExists(enemy.Pos, g.Player.Pos) {
		return stepToward(enemy.Pos, g.Player.Pos)
	}
	enemy.PatternTick++
	switch enemy.Kind {
	case EnemySnake:
		if enemy.PatternTick%2 == 0 {
			return Point{enemy.Pos.X + 1, enemy.Pos.Y + 1}
		}
		return Point{enemy.Pos.X - 1, enemy.Pos.Y + 1}
	case EnemyOgre:
		if enemy.PatternTick%2 == 0 {
			return Point{enemy.Pos.X + 2, enemy.Pos.Y}
		}
		return Point{enemy.Pos.X - 2, enemy.Pos.Y}
	case EnemyGhost:
		enemy.Invisible = enemy.PatternTick%3 == 0
		if enemy.PatternTick%4 == 0 {
			return g.randomRoomPoint(enemy.RoomID)
		}
	}
	dirs := []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	d := dirs[g.rng().Intn(len(dirs))]
	return Point{enemy.Pos.X + d.X, enemy.Pos.Y + d.Y}
}

func (g *Game) pickupAt(pos Point) {
	for i, gi := range g.Level.Items {
		if gi.Pos == pos {
			if gi.Item.Kind == ItemTreasure {
				g.Stats.Treasure += gi.Item.Value
			} else if gi.Item.Kind == ItemKey {
				g.Player.Bag.Add(gi.Item)
				g.Message = "Ключ подобран: " + gi.Item.SubType + "."
			} else if !g.Player.Bag.Add(gi.Item) {
				g.Message = "Рюкзак полон."
				return
			} else {
				g.Message = "Предмет добавлен в рюкзак."
			}
			g.Level.Items = append(g.Level.Items[:i], g.Level.Items[i+1:]...)
			return
		}
	}
	g.Message = "Вы сделали ход."
}

func (g *Game) descend() {
	if g.Level.Depth >= MaxDepth {
		g.State = StateWon
		g.Message = "Вы прошли все 21 уровней подземелья."
		return
	}
	nextDepth := g.Level.Depth + 1
	g.adjustBalance()
	g.Level = GenerateLevelWithBalance(nextDepth, g.Seed, g.Balance)
	g.Player.Pos = g.startPoint()
	g.Stats.ReachedLevel = nextDepth
	g.Message = fmt.Sprintf("Вы спустились на уровень %d.", nextDepth)
	g.revealAroundPlayer()
}

func (g *Game) applyItemStats(item Item) {
	g.Player.MaxHealth += item.MaxHealth
	g.Player.Health += item.MaxHealth + item.Health
	g.Player.Agility += item.Agility
	g.Player.Strength += item.Strength
	if g.Player.Health > g.Player.MaxHealth {
		g.Player.Health = g.Player.MaxHealth
	}
}

func (g *Game) tickEffects() {
	kept := g.Player.Effects[:0]
	for _, effect := range g.Player.Effects {
		effect.Turns--
		if effect.Turns <= 0 {
			switch effect.Stat {
			case StatMaxHealth:
				g.Player.MaxHealth -= effect.Value
				if g.Player.MaxHealth < 1 {
					g.Player.MaxHealth = 1
				}
				if g.Player.Health > g.Player.MaxHealth {
					g.Player.Health = max(1, g.Player.MaxHealth)
				}
			case StatAgility:
				g.Player.Agility -= effect.Value
			case StatStrength:
				g.Player.Strength -= effect.Value
			}
			if g.Player.Health <= 0 {
				g.Player.Health = 1
			}
			continue
		}
		kept = append(kept, effect)
	}
	g.Player.Effects = kept
}

func itemEffects(item Item, turns int) []TimedEffect {
	var effects []TimedEffect
	if item.MaxHealth != 0 {
		effects = append(effects, TimedEffect{Stat: StatMaxHealth, Value: item.MaxHealth, Turns: turns})
	}
	if item.Agility != 0 {
		effects = append(effects, TimedEffect{Stat: StatAgility, Value: item.Agility, Turns: turns})
	}
	if item.Strength != 0 {
		effects = append(effects, TimedEffect{Stat: StatStrength, Value: item.Strength, Turns: turns})
	}
	return effects
}

func (g *Game) dropNearPlayer(item Item) {
	for _, d := range []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		p := Point{g.Player.Pos.X + d.X, g.Player.Pos.Y + d.Y}
		if g.inBounds(p) && g.isWalkable(p) && g.enemyAt(p) < 0 {
			g.Level.Items = append(g.Level.Items, GroundItem{Item: item, Pos: p})
			return
		}
	}
}

func (g *Game) revealAroundPlayer() {
	for y := 0; y < g.Level.Height; y++ {
		for x := 0; x < g.Level.Width; x++ {
			p := Point{x, y}
			if distance(g.Player.Pos, p) <= 8 && g.lineOfSight(g.Player.Pos, p) {
				g.Level.Explored[y][x] = true
			}
		}
	}
	for _, room := range g.Level.Rooms {
		if room.Contains(g.Player.Pos) {
			for y := room.Y; y < room.Y+room.H; y++ {
				for x := room.X; x < room.X+room.W; x++ {
					g.Level.Explored[y][x] = true
				}
			}
		}
	}
}

func (g *Game) tileAt(p Point) Tile {
	return g.Level.Tiles[p.Y][p.X]
}

func (g *Game) inBounds(p Point) bool {
	return p.X >= 0 && p.X < g.Level.Width && p.Y >= 0 && p.Y < g.Level.Height
}

func (g *Game) isWalkable(p Point) bool {
	t := g.tileAt(p)
	if t == TileDoor {
		for i := range g.Level.Doors {
			door := &g.Level.Doors[i]
			if door.Pos == p {
				if !door.Locked || g.Player.Bag.Keys[door.Color] {
					door.Locked = false
					return true
				}
				g.Message = "Нужен ключ: " + string(door.Color) + "."
				return false
			}
		}
		return true
	}
	return t == TileFloor || t == TileCorridor || t == TileExit
}

func (g *Game) enemyAt(p Point) int {
	for i, e := range g.Level.Enemies {
		if e.Health > 0 && e.Pos == p {
			return i
		}
	}
	return -1
}

func (g *Game) rng() *rand.Rand {
	return rand.New(rand.NewSource(g.Seed + int64(g.Turn+1)*104729 + int64(g.Level.Depth)*1009))
}

func hit(attackerAgility, defenderAgility int, rng *rand.Rand) bool {
	chance := 55 + (attackerAgility-defenderAgility)*5
	if chance < 10 {
		chance = 10
	}
	if chance > 92 {
		chance = 92
	}
	return rng.Intn(100) < chance
}

func enemyTreasure(enemy Enemy, rng *rand.Rand) int {
	base := enemy.Hostility + enemy.Strength + enemy.Agility + enemy.MaxHealth/2
	return max(1, base/3+rng.Intn(max(1, base/2)))
}

func adjacent(a, b Point) bool {
	return int(math.Abs(float64(a.X-b.X)))+int(math.Abs(float64(a.Y-b.Y))) == 1
}

func distance(a, b Point) float64 {
	return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))
}

func stepToward(a, b Point) Point {
	n := a
	if abs(b.X-a.X) > abs(b.Y-a.Y) {
		if b.X > a.X {
			n.X++
		} else {
			n.X--
		}
	} else if b.Y != a.Y {
		if b.Y > a.Y {
			n.Y++
		} else {
			n.Y--
		}
	}
	return n
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func enemyLabel(kind EnemyKind) string {
	switch kind {
	case EnemyZombie:
		return "Зомби"
	case EnemyVampire:
		return "Вампир"
	case EnemyGhost:
		return "Привидение"
	case EnemyOgre:
		return "Огр"
	case EnemySnake:
		return "Змей-маг"
	case EnemyMimic:
		return "Мимик"
	default:
		return "Враг"
	}
}

func (g *Game) pathExists(from, to Point) bool {
	seen := map[Point]bool{from: true}
	queue := []Point{from}
	dirs := []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p == to {
			return true
		}
		for _, d := range dirs {
			n := Point{p.X + d.X, p.Y + d.Y}
			if !g.inBounds(n) || seen[n] || !g.pathWalkable(n) {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return false
}

func (g *Game) pathWalkable(p Point) bool {
	t := g.tileAt(p)
	if t == TileDoor {
		for _, door := range g.Level.Doors {
			if door.Pos == p {
				return !door.Locked || g.Player.Bag.Keys[door.Color]
			}
		}
		return true
	}
	return t == TileFloor || t == TileCorridor || t == TileExit
}

func (g *Game) lineOfSight(a, b Point) bool {
	x0, y0, x1, y1 := a.X, a.Y, b.X, b.Y
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		p := Point{x0, y0}
		if p != a && p != b && g.tileAt(p) == TileWall {
			return false
		}
		if x0 == x1 && y0 == y1 {
			return true
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func (g *Game) randomRoomPoint(roomID int) Point {
	if roomID < 0 || roomID >= len(g.Level.Rooms) {
		return g.Player.Pos
	}
	return randomPointInRoom(g.Level.Rooms[roomID], g.rng())
}

func (g *Game) startPoint() Point {
	room := g.Level.Rooms[g.Level.StartRoom]
	rng := rand.New(rand.NewSource(g.Seed + int64(g.Level.Depth)*3571))
	for i := 0; i < 50; i++ {
		p := randomPointInRoom(room, rng)
		if g.inBounds(p) && g.isWalkable(p) {
			return p
		}
	}
	return room.Center
}

func (g *Game) adjustBalance() {
	healthRatio := float64(g.Player.Health) / float64(max(1, g.Player.MaxHealth))
	if healthRatio > 0.75 && g.Stats.HitsTaken < g.Level.Depth*2 {
		g.Balance++
	} else if healthRatio < 0.35 || g.Stats.FoodEaten > g.Level.Depth/2+1 {
		g.Balance--
	}
	if g.Balance > 3 {
		g.Balance = 3
	}
	if g.Balance < -2 {
		g.Balance = -2
	}
}
