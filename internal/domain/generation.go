package domain

import "math/rand"

func GenerateLevel(depth int, seed int64) Level {
	return GenerateLevelWithBalance(depth, seed, 0)
}

func GenerateLevelWithBalance(depth int, seed int64, balance int) Level {
	rng := rand.New(rand.NewSource(seed + int64(depth)*7919))
	level := Level{
		Depth: depth, Width: BoardWidth, Height: BoardHeight,
		Tiles:     makeTiles(BoardWidth, BoardHeight, TileWall),
		Explored:  makeBoolGrid(BoardWidth, BoardHeight),
		StartRoom: rng.Intn(9),
	}
	level.ExitRoom = rng.Intn(9)
	for level.ExitRoom == level.StartRoom {
		level.ExitRoom = rng.Intn(9)
	}
	sectionW := BoardWidth / 3
	sectionH := (BoardHeight - 1) / 3
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			id := row*3 + col
			w := 8 + rng.Intn(10)
			h := 4 + rng.Intn(4)
			minX := col*sectionW + 2
			minY := row*sectionH + 1
			maxX := (col+1)*sectionW - w - 2
			maxY := (row+1)*sectionH - h - 1
			if maxX < minX {
				maxX = minX
			}
			if maxY < minY {
				maxY = minY
			}
			room := Room{ID: id, X: minX + rng.Intn(maxX-minX+1), Y: minY + rng.Intn(maxY-minY+1), W: w, H: h}
			room.Center = Point{room.X + room.W/2, room.Y + room.H/2}
			room.Started = id == level.StartRoom
			room.Exit = id == level.ExitRoom
			level.Rooms = append(level.Rooms, room)
			carveRoom(&level, room)
		}
	}
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			id := row*3 + col
			if col < 2 {
				carveCorridor(&level, level.Rooms[id].Center, level.Rooms[id+1].Center)
			}
			if row < 2 {
				carveCorridor(&level, level.Rooms[id].Center, level.Rooms[id+3].Center)
			}
		}
	}
	exit := level.Rooms[level.ExitRoom].Center
	level.Tiles[exit.Y][exit.X] = TileExit
	generateDoorsAndKeys(&level, rng)
	populateLevel(&level, rng, balance)
	if !keysAndDoorsAreSolvable(level) {
		level.Doors = nil
		for y := range level.Tiles {
			for x := range level.Tiles[y] {
				if level.Tiles[y][x] == TileDoor {
					level.Tiles[y][x] = TileCorridor
				}
			}
		}
	}
	return level
}

func makeTiles(w, h int, tile Tile) [][]Tile {
	grid := make([][]Tile, h)
	for y := range grid {
		grid[y] = make([]Tile, w)
		for x := range grid[y] {
			grid[y][x] = tile
		}
	}
	return grid
}

func makeBoolGrid(w, h int) [][]bool {
	grid := make([][]bool, h)
	for y := range grid {
		grid[y] = make([]bool, w)
	}
	return grid
}

func carveRoom(level *Level, room Room) {
	for y := room.Y; y < room.Y+room.H; y++ {
		for x := room.X; x < room.X+room.W; x++ {
			if x == room.X || y == room.Y || x == room.X+room.W-1 || y == room.Y+room.H-1 {
				level.Tiles[y][x] = TileWall
			} else {
				level.Tiles[y][x] = TileFloor
			}
		}
	}
}

func carveCorridor(level *Level, a, b Point) {
	p := a
	stepX := 1
	if b.X < p.X {
		stepX = -1
	}
	for p.X != b.X {
		if level.Tiles[p.Y][p.X] == TileWall {
			level.Tiles[p.Y][p.X] = TileCorridor
		}
		level.Corridors = append(level.Corridors, p)
		p.X += stepX
	}
	stepY := 1
	if b.Y < p.Y {
		stepY = -1
	}
	for p.Y != b.Y {
		if level.Tiles[p.Y][p.X] == TileWall {
			level.Tiles[p.Y][p.X] = TileCorridor
		}
		level.Corridors = append(level.Corridors, p)
		p.Y += stepY
	}
	if level.Tiles[p.Y][p.X] == TileWall {
		level.Tiles[p.Y][p.X] = TileCorridor
	}
	level.Corridors = append(level.Corridors, p)
}

func populateLevel(level *Level, rng *rand.Rand, balance int) {
	for _, room := range level.Rooms {
		if room.ID == level.StartRoom {
			continue
		}
		enemyCount := max(1, 1+level.Depth/5+balance)
		if rng.Intn(100) < 35+level.Depth*2+balance*8 {
			enemyCount++
		}
		for i := 0; i < enemyCount; i++ {
			level.Enemies = append(level.Enemies, newEnemy(enemyKindForDepth(level.Depth, rng), randomPointInRoom(room, rng), room.ID, len(level.Enemies)+1))
		}
		itemChance := 75 - level.Depth*2 - balance*8
		if balance < 0 {
			itemChance += 25
		}
		if rng.Intn(100) < itemChance {
			level.Items = append(level.Items, GroundItem{Item: randomUsefulItem(level.Depth, rng), Pos: randomPointInRoom(room, rng)})
		}
	}
}

func generateDoorsAndKeys(level *Level, rng *rand.Rand) {
	colors := []DoorColor{DoorRed, DoorBlue, DoorYellow}
	count := 1 + level.Depth/8
	if count > len(colors) {
		count = len(colors)
	}
	roomOrder := reachableRoomOrder(*level)
	if len(roomOrder) < 2 {
		return
	}
	for i := 0; i < count; i++ {
		targetIndex := 1 + (i+1)*(len(roomOrder)-1)/(count+1)
		targetRoom := roomOrder[targetIndex]
		doorPos, ok := doorPositionForRoom(*level, targetRoom)
		if !ok {
			continue
		}
		color := colors[i]
		level.Tiles[doorPos.Y][doorPos.X] = TileDoor
		level.Doors = append(level.Doors, Door{Pos: doorPos, Color: color, Locked: true})
		keyRoom := roomOrder[max(0, targetIndex-1)]
		keyPos := randomPointInRoom(level.Rooms[keyRoom], rng)
		level.Items = append(level.Items, GroundItem{Item: Item{Kind: ItemKey, SubType: string(color)}, Pos: keyPos})
	}
}

func reachableRoomOrder(level Level) []int {
	seen := map[int]bool{level.StartRoom: true}
	order := []int{level.StartRoom}
	queue := []int{level.StartRoom}
	for len(queue) > 0 {
		roomID := queue[0]
		queue = queue[1:]
		for _, n := range neighborRooms(roomID) {
			if n < 0 || n >= len(level.Rooms) || seen[n] {
				continue
			}
			if roomCentersConnectedIgnoringDoors(level, level.Rooms[roomID].Center, level.Rooms[n].Center) {
				seen[n] = true
				order = append(order, n)
				queue = append(queue, n)
			}
		}
	}
	return order
}

func neighborRooms(id int) []int {
	var out []int
	row, col := id/3, id%3
	if col > 0 {
		out = append(out, id-1)
	}
	if col < 2 {
		out = append(out, id+1)
	}
	if row > 0 {
		out = append(out, id-3)
	}
	if row < 2 {
		out = append(out, id+3)
	}
	return out
}

func doorPositionForRoom(level Level, roomID int) (Point, bool) {
	room := level.Rooms[roomID]
	best := Point{}
	bestDist := 1 << 30
	for _, p := range level.Corridors {
		if p.X < room.X-1 || p.X > room.X+room.W || p.Y < room.Y-1 || p.Y > room.Y+room.H {
			continue
		}
		if level.Tiles[p.Y][p.X] != TileCorridor {
			continue
		}
		d := abs(p.X-room.Center.X) + abs(p.Y-room.Center.Y)
		if d < bestDist {
			best = p
			bestDist = d
		}
	}
	return best, bestDist != 1<<30
}

func roomCentersConnectedIgnoringDoors(level Level, a, b Point) bool {
	seen := map[Point]bool{a: true}
	queue := []Point{a}
	dirs := []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		if p == b {
			return true
		}
		for _, d := range dirs {
			n := Point{p.X + d.X, p.Y + d.Y}
			if n.X < 0 || n.X >= level.Width || n.Y < 0 || n.Y >= level.Height || seen[n] {
				continue
			}
			t := level.Tiles[n.Y][n.X]
			if t != TileFloor && t != TileCorridor && t != TileExit && t != TileDoor {
				continue
			}
			seen[n] = true
			queue = append(queue, n)
		}
	}
	return false
}

func keysAndDoorsAreSolvable(level Level) bool {
	keys := map[DoorColor]bool{}
	seen := map[Point]bool{level.Rooms[level.StartRoom].Center: true}
	queue := []Point{level.Rooms[level.StartRoom].Center}
	progress := true
	for progress {
		progress = false
		for len(queue) > 0 {
			p := queue[0]
			queue = queue[1:]
			for _, item := range level.Items {
				if item.Pos == p && item.Item.Kind == ItemKey {
					color := DoorColor(item.Item.SubType)
					if !keys[color] {
						keys[color] = true
						progress = true
					}
				}
			}
			for _, d := range []Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				n := Point{p.X + d.X, p.Y + d.Y}
				if n.X < 0 || n.X >= level.Width || n.Y < 0 || n.Y >= level.Height || seen[n] {
					continue
				}
				if !walkableWithKeys(level, n, keys) {
					continue
				}
				seen[n] = true
				queue = append(queue, n)
			}
		}
		if progress {
			for p := range seen {
				queue = append(queue, p)
			}
		}
	}
	return seen[level.Rooms[level.ExitRoom].Center]
}

func walkableWithKeys(level Level, p Point, keys map[DoorColor]bool) bool {
	t := level.Tiles[p.Y][p.X]
	if t == TileDoor {
		for _, door := range level.Doors {
			if door.Pos == p {
				return !door.Locked || keys[door.Color]
			}
		}
		return true
	}
	return t == TileFloor || t == TileCorridor || t == TileExit
}

func enemyKindForDepth(depth int, rng *rand.Rand) EnemyKind {
	kinds := []EnemyKind{EnemyZombie, EnemyGhost, EnemySnake}
	if depth > 3 {
		kinds = append(kinds, EnemyVampire)
	}
	if depth > 6 {
		kinds = append(kinds, EnemyOgre)
	}
	if depth > 10 {
		kinds = append(kinds, EnemyMimic)
	}
	return kinds[rng.Intn(len(kinds))]
}

func newEnemy(kind EnemyKind, pos Point, roomID, id int) Enemy {
	e := Enemy{ID: id, Kind: kind, Pos: pos, RoomID: roomID}
	switch kind {
	case EnemyZombie:
		e.MaxHealth, e.Health, e.Agility, e.Strength, e.Hostility = 20, 20, 4, 7, 6
	case EnemyVampire:
		e.MaxHealth, e.Health, e.Agility, e.Strength, e.Hostility, e.FirstMiss = 18, 18, 12, 7, 10, true
	case EnemyGhost:
		e.MaxHealth, e.Health, e.Agility, e.Strength, e.Hostility = 10, 10, 13, 4, 4
	case EnemyOgre:
		e.MaxHealth, e.Health, e.Agility, e.Strength, e.Hostility = 30, 30, 3, 14, 6
	case EnemySnake:
		e.MaxHealth, e.Health, e.Agility, e.Strength, e.Hostility = 14, 14, 15, 6, 12
	case EnemyMimic:
		e.MaxHealth, e.Health, e.Agility, e.Strength, e.Hostility = 22, 22, 12, 5, 3
	}
	return e
}

func randomUsefulItem(depth int, rng *rand.Rand) Item {
	switch rng.Intn(4) {
	case 0:
		return Item{Kind: ItemFood, SubType: "ration", Health: 7 + rng.Intn(8)}
	case 1:
		return Item{Kind: ItemElixir, SubType: string([]StatKind{StatAgility, StatStrength, StatMaxHealth}[rng.Intn(3)]), Agility: rng.Intn(2), Strength: 1 + rng.Intn(3), MaxHealth: rng.Intn(3), Duration: 20}
	case 2:
		return Item{Kind: ItemScroll, SubType: string([]StatKind{StatAgility, StatStrength, StatMaxHealth}[rng.Intn(3)]), Agility: rng.Intn(2), Strength: 1 + rng.Intn(2), MaxHealth: 1 + rng.Intn(4)}
	default:
		return Item{Kind: ItemWeapon, SubType: "blade", Strength: 2 + depth/4 + rng.Intn(4)}
	}
}

func randomPointInRoom(room Room, rng *rand.Rand) Point {
	return Point{X: room.X + 1 + rng.Intn(max(1, room.W-2)), Y: room.Y + 1 + rng.Intn(max(1, room.H-2))}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
