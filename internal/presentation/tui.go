package presentation

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"rogue/internal/datalayer"
	"rogue/internal/domain"
)

type App struct {
	game     *domain.Game
	store    datalayer.Store
	screen   tcell.Screen
	recorded bool
	view3D   bool
	facing   int
}

func NewApp(game *domain.Game, store datalayer.Store) *App {
	return &App{game: game, store: store}
}

func (a *App) Run() error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	a.screen = screen
	defer screen.Fini()
	a.drawMenu("N - новая игра, R - продолжить, L - рекорды, Q - выход")
	for {
		ev := screen.PollEvent()
		key, r := keyRune(ev)
		switch {
		case key == tcell.KeyEscape || r == 'q':
			return nil
		case r == 'n':
			a.game.NewRun(time.Now().UnixNano())
			a.recorded = false
			return a.gameLoop()
		case r == 'r':
			state, err := a.store.LoadGame()
			if err != nil {
				a.drawMenu("Не удалось загрузить сохранение: " + err.Error())
				continue
			}
			a.game = domain.LoadFromSnapshot(state)
			a.recorded = false
			return a.gameLoop()
		case r == 'l':
			a.drawLeaderboard()
		}
	}
}

func (a *App) gameLoop() error {
	a.drawGame()
	for {
		ev := a.screen.PollEvent()
		key, r := keyRune(ev)
		if key == tcell.KeyEscape || r == 'q' {
			return a.store.SaveGame(a.game.Snapshot())
		}
		switch r {
		case 'w':
			if a.view3D {
				a.moveForward(1)
			} else {
				a.move(0, -1)
			}
		case 'a':
			if a.view3D {
				a.facing = (a.facing + 3) % 4
			} else {
				a.move(-1, 0)
			}
		case 's':
			if a.view3D {
				a.moveForward(-1)
			} else {
				a.move(0, 1)
			}
		case 'd':
			if a.view3D {
				a.facing = (a.facing + 1) % 4
			} else {
				a.move(1, 0)
			}
		case 'h':
			a.useFromInventory(domain.ItemWeapon)
		case 'j':
			a.useFromInventory(domain.ItemFood)
		case 'k':
			a.useFromInventory(domain.ItemElixir)
		case 'e':
			a.useFromInventory(domain.ItemScroll)
		case 'l':
			a.drawLeaderboard()
			a.waitAnyKey()
		case 'p':
			if err := a.store.SaveGame(a.game.Snapshot()); err != nil {
				a.game.Message = "Ошибка сохранения: " + err.Error()
			} else {
				a.game.Message = "Игра сохранена."
			}
		case 'v':
			a.view3D = !a.view3D
		}
		a.drawGame()
	}
}

func (a *App) moveForward(sign int) {
	d := facingDelta(a.facing)
	a.move(d.X*sign, d.Y*sign)
}

func (a *App) move(dx, dy int) {
	oldDepth := a.game.Level.Depth
	result := a.game.Move(dx, dy)
	if a.game.Level.Depth > oldDepth && a.game.State == domain.StatePlaying {
		if err := a.store.SaveGame(a.game.Snapshot()); err != nil {
			a.game.Message = "Ошибка автосохранения: " + err.Error()
		}
	}
	a.afterAction(result)
}

func (a *App) afterAction(result domain.EventResult) {
	if result.Done && !a.recorded {
		record := a.game.CurrentRecord()
		record.Finished = time.Now()
		if err := a.store.AppendRecord(record); err != nil {
			a.game.Message = "Ошибка записи результата: " + err.Error()
			return
		}
		a.recorded = true
		if a.game.State == domain.StateDead {
			a.game.NewRun(time.Now().UnixNano())
			a.game.Message = "Герой погиб. Результат записан, начат новый забег."
			a.recorded = false
		}
		if err := a.store.SaveGame(a.game.Snapshot()); err != nil {
			a.game.Message = "Ошибка сохранения: " + err.Error()
		}
	}
}

func (a *App) useFromInventory(kind domain.ItemKind) {
	items := a.game.Inventory(kind)
	if kind == domain.ItemWeapon && a.game.Player.Weapon != nil {
		items = append([]domain.Item{{Kind: domain.ItemWeapon, SubType: "убрать оружие"}}, items...)
	}
	a.drawInventory(kind, items)
	for {
		ev := a.screen.PollEvent()
		key, r := keyRune(ev)
		if key == tcell.KeyEscape {
			a.game.Message = "Выбор отменен."
			return
		}
		if kind == domain.ItemWeapon && r == '0' {
			a.afterAction(a.game.UseItem(kind, -1))
			return
		}
		if r >= '1' && r <= '9' {
			idx := int(r - '1')
			a.afterAction(a.game.UseItem(kind, idx))
			return
		}
	}
}

func (a *App) drawMenu(message string) {
	a.screen.Clear()
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorBlack)
	drawText(a.screen, 2, 1, style.Bold(true), "ROGUE 1980 GO")
	drawText(a.screen, 2, 3, style, message)
	drawText(a.screen, 2, 5, style, "Управление в игре: WASD ход, H оружие, J еда, K эликсир, E свиток, P сохранить, L рекорды, Q выход.")
	a.screen.Show()
}

func (a *App) drawGame() {
	a.screen.Clear()
	if a.view3D {
		a.draw3D()
		a.drawMiniMap()
	} else {
		a.drawMap()
	}
	a.drawStatus()
	a.screen.Show()
}

func (a *App) drawMap() {
	currentRoom := currentRoomID(a.game)
	for y := 0; y < a.game.Level.Height; y++ {
		for x := 0; x < a.game.Level.Width; x++ {
			p := domain.Point{X: x, Y: y}
			style := tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray)
			ch := ' '
			if a.game.Level.Explored[y][x] {
				roomID := roomAt(a.game, p)
				if roomID >= 0 && roomID != currentRoom && a.game.Level.Tiles[y][x] != domain.TileWall {
					ch = ' '
					style = tcell.StyleDefault.Foreground(tcell.ColorBlack)
				} else {
					ch, style = tileGlyph(a.game.Level.Tiles[y][x])
				}
			}
			a.screen.SetContent(x, y, ch, nil, style)
			if a.game.Level.Explored[y][x] && roomAt(a.game, p) == currentRoom {
				if item, ok := itemAt(a.game.Level.Items, p); ok {
					a.screen.SetContent(x, y, itemGlyph(item.Kind), nil, itemStyle(item.Kind))
				}
			}
		}
	}
	for _, e := range a.game.Level.Enemies {
		if e.Invisible || !a.game.Level.Explored[e.Pos.Y][e.Pos.X] || roomAt(a.game, e.Pos) != currentRoom {
			continue
		}
		a.screen.SetContent(e.Pos.X, e.Pos.Y, enemyGlyph(e.Kind), nil, enemyStyle(e.Kind))
	}
	a.screen.SetContent(a.game.Player.Pos.X, a.game.Player.Pos.Y, '@', nil, tcell.StyleDefault.Foreground(tcell.ColorAqua).Bold(true))
}

func (a *App) draw3D() {
	width := domain.BoardWidth
	height := domain.BoardHeight
	for x := 0; x < width; x++ {
		rayAngle := facingAngle(a.facing) - math.Pi/6 + (float64(x)/float64(width))*math.Pi/3
		dist, tile := a.castRay(rayAngle)
		if dist < 0.25 {
			dist = 0.25
		}
		wallHeight := int(float64(height) / dist)
		if wallHeight > height {
			wallHeight = height
		}
		top := (height - wallHeight) / 2
		bottom := top + wallHeight
		for y := 0; y < height; y++ {
			ch := ' '
			style := tcell.StyleDefault.Foreground(tcell.ColorBlack)
			switch {
			case y < top:
				ch = '.'
				style = tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray)
			case y >= bottom:
				ch = ','
				style = tcell.StyleDefault.Foreground(tcell.ColorGray)
			default:
				ch = texturedWallGlyph(x, y, tile, dist)
				style = wallStyle(tile, dist)
			}
			a.screen.SetContent(x, y, ch, nil, style)
		}
	}
}

func (a *App) castRay(angle float64) (float64, domain.Tile) {
	px := float64(a.game.Player.Pos.X) + 0.5
	py := float64(a.game.Player.Pos.Y) + 0.5
	dx := math.Cos(angle)
	dy := math.Sin(angle)
	for dist := 0.05; dist < 18; dist += 0.05 {
		x := int(px + dx*dist)
		y := int(py + dy*dist)
		if x < 0 || x >= a.game.Level.Width || y < 0 || y >= a.game.Level.Height {
			return dist, domain.TileWall
		}
		tile := a.game.Level.Tiles[y][x]
		if tile == domain.TileWall || tile == domain.TileDoor {
			return dist, tile
		}
	}
	return 18, domain.TileWall
}

func (a *App) drawMiniMap() {
	offsetX, offsetY := 1, 1
	maxW, maxH := 24, 10
	startX := a.game.Player.Pos.X - maxW/2
	startY := a.game.Player.Pos.Y - maxH/2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}
	if startX+maxW > a.game.Level.Width {
		startX = max(0, a.game.Level.Width-maxW)
	}
	if startY+maxH > a.game.Level.Height {
		startY = max(0, a.game.Level.Height-maxH)
	}
	for sy := 0; sy < maxH && startY+sy < a.game.Level.Height; sy++ {
		for sx := 0; sx < maxW && startX+sx < a.game.Level.Width; sx++ {
			x, y := startX+sx, startY+sy
			p := domain.Point{X: x, Y: y}
			ch := ' '
			style := tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray)
			if a.game.Level.Explored[y][x] {
				ch, style = tileGlyph(a.game.Level.Tiles[y][x])
			}
			if item, ok := itemAt(a.game.Level.Items, p); ok && a.game.Level.Explored[y][x] {
				ch = itemGlyph(item.Kind)
				style = itemStyle(item.Kind)
			}
			a.screen.SetContent(offsetX+sx, offsetY+sy, ch, nil, style.Background(tcell.ColorBlack))
		}
	}
	px, py := a.game.Player.Pos.X-startX, a.game.Player.Pos.Y-startY
	if px >= 0 && px < maxW && py >= 0 && py < maxH {
		a.screen.SetContent(offsetX+px, offsetY+py, facingGlyph(a.facing), nil, tcell.StyleDefault.Foreground(tcell.ColorAqua).Bold(true))
	}
}

func currentRoomID(g *domain.Game) int {
	return roomAt(g, g.Player.Pos)
}

func roomAt(g *domain.Game, p domain.Point) int {
	for _, room := range g.Level.Rooms {
		if room.Contains(p) {
			return room.ID
		}
	}
	return -1
}

func (a *App) drawStatus() {
	y := domain.BoardHeight + 1
	weapon := "нет"
	if a.game.Player.Weapon != nil {
		weapon = fmt.Sprintf("%s +%d", a.game.Player.Weapon.SubType, a.game.Player.Weapon.Strength)
	}
	status := fmt.Sprintf("Ур.%d/%d HP %d/%d AGI %d STR %d Золото %d Враги %d Оружие %s",
		a.game.Level.Depth, domain.MaxDepth, a.game.Player.Health, a.game.Player.MaxHealth,
		a.game.Player.Agility, a.game.Player.Strength, a.game.Stats.Treasure, len(a.game.Level.Enemies), weapon)
	drawText(a.screen, 0, y, tcell.StyleDefault.Foreground(tcell.ColorWhite), pad(status, domain.BoardWidth))
	drawText(a.screen, 0, y+1, tcell.StyleDefault.Foreground(tcell.ColorYellow), pad(a.game.Message, domain.BoardWidth))
	mode := "2D"
	if a.view3D {
		mode = "3D: W/S вперед/назад, A/D поворот"
	}
	drawText(a.screen, 0, y+2, tcell.StyleDefault.Foreground(tcell.ColorGray), "WASD | H оружие | J еда | K эликсир | E свиток | V вид | P сохранить | L рекорды | Q выход | "+mode)
	if a.game.State != domain.StatePlaying {
		drawText(a.screen, 0, y+4, tcell.StyleDefault.Foreground(tcell.ColorRed).Bold(true), "Забег завершен. Нажмите Q для выхода.")
	}
}

func (a *App) drawInventory(kind domain.ItemKind, items []domain.Item) {
	a.screen.Clear()
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	drawText(a.screen, 2, 1, style.Bold(true), "Инвентарь: "+string(kind))
	if kind == domain.ItemWeapon && a.game.Player.Weapon != nil {
		drawText(a.screen, 2, 3, style, "0. Убрать оружие из рук")
	}
	startY := 4
	for i, item := range a.game.Inventory(kind) {
		drawText(a.screen, 2, startY+i, style, fmt.Sprintf("%d. %s", i+1, describeItem(item)))
	}
	if len(items) == 0 {
		drawText(a.screen, 2, startY, style, "Пусто. Esc - назад.")
	} else {
		drawText(a.screen, 2, startY+len(items)+1, style, "Выберите 1-9, для оружия 0 убирает предмет из рук. Esc - назад.")
	}
	a.screen.Show()
}

func (a *App) drawLeaderboard() {
	a.screen.Clear()
	records, err := a.store.Records()
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	drawText(a.screen, 2, 1, style.Bold(true), "Таблица рекордов")
	if err != nil {
		drawText(a.screen, 2, 3, style, "Ошибка чтения рекордов: "+err.Error())
		a.screen.Show()
		return
	}
	if len(records) == 0 {
		drawText(a.screen, 2, 3, style, "Рекордов пока нет.")
	} else {
		for i, record := range records {
			if i >= 12 {
				break
			}
			outcome := "поражение"
			if record.Won {
				outcome = "победа"
			}
			line := fmt.Sprintf("%2d. золото=%d уровень=%d враги=%d еда=%d эликсиры=%d свитки=%d удары=%d/%d шаги=%d %s",
				i+1, record.Stats.Treasure, record.Stats.ReachedLevel, record.Stats.Defeated,
				record.Stats.FoodEaten, record.Stats.ElixirsDrunk, record.Stats.ScrollsRead,
				record.Stats.HitsDealt, record.Stats.HitsTaken, record.Stats.CellsWalked, outcome)
			drawText(a.screen, 2, 3+i, style, line)
		}
	}
	drawText(a.screen, 2, 18, style, "Нажмите любую клавишу.")
	a.screen.Show()
}

func (a *App) waitAnyKey() {
	a.screen.PollEvent()
}

func keyRune(ev tcell.Event) (tcell.Key, rune) {
	keyEvent, ok := ev.(*tcell.EventKey)
	if !ok {
		return tcell.KeyRune, 0
	}
	r := keyEvent.Rune()
	if r >= 'A' && r <= 'Z' {
		r += 'a' - 'A'
	}
	return keyEvent.Key(), r
}

func drawText(screen tcell.Screen, x, y int, style tcell.Style, text string) {
	for i, r := range []rune(text) {
		screen.SetContent(x+i, y, r, nil, style)
	}
}

func tileGlyph(tile domain.Tile) (rune, tcell.Style) {
	switch tile {
	case domain.TileFloor:
		return '.', tcell.StyleDefault.Foreground(tcell.ColorGray)
	case domain.TileCorridor:
		return '#', tcell.StyleDefault.Foreground(tcell.ColorDarkCyan)
	case domain.TileExit:
		return '>', tcell.StyleDefault.Foreground(tcell.ColorGreen).Bold(true)
	case domain.TileDoor:
		return '+', tcell.StyleDefault.Foreground(tcell.ColorOrange).Bold(true)
	default:
		return '#', tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray)
	}
}

func itemAt(items []domain.GroundItem, p domain.Point) (domain.Item, bool) {
	for _, item := range items {
		if item.Pos == p {
			return item.Item, true
		}
	}
	return domain.Item{}, false
}

func itemGlyph(kind domain.ItemKind) rune {
	switch kind {
	case domain.ItemFood:
		return '%'
	case domain.ItemElixir:
		return '!'
	case domain.ItemScroll:
		return '?'
	case domain.ItemWeapon:
		return ')'
	case domain.ItemKey:
		return '&'
	case domain.ItemTreasure:
		return '$'
	default:
		return '*'
	}
}

func itemStyle(kind domain.ItemKind) tcell.Style {
	switch kind {
	case domain.ItemFood:
		return tcell.StyleDefault.Foreground(tcell.ColorOlive)
	case domain.ItemElixir:
		return tcell.StyleDefault.Foreground(tcell.ColorPurple)
	case domain.ItemScroll:
		return tcell.StyleDefault.Foreground(tcell.ColorSilver)
	case domain.ItemWeapon:
		return tcell.StyleDefault.Foreground(tcell.ColorOrange)
	case domain.ItemKey:
		return tcell.StyleDefault.Foreground(tcell.ColorBlue).Bold(true)
	case domain.ItemTreasure:
		return tcell.StyleDefault.Foreground(tcell.ColorGold)
	default:
		return tcell.StyleDefault.Foreground(tcell.ColorWhite)
	}
}

func facingDelta(facing int) domain.Point {
	switch facing {
	case 0:
		return domain.Point{X: 0, Y: -1}
	case 1:
		return domain.Point{X: 1, Y: 0}
	case 2:
		return domain.Point{X: 0, Y: 1}
	default:
		return domain.Point{X: -1, Y: 0}
	}
}

func facingAngle(facing int) float64 {
	switch facing {
	case 0:
		return -math.Pi / 2
	case 1:
		return 0
	case 2:
		return math.Pi / 2
	default:
		return math.Pi
	}
}

func facingGlyph(facing int) rune {
	switch facing {
	case 0:
		return '^'
	case 1:
		return '>'
	case 2:
		return 'v'
	default:
		return '<'
	}
}

func texturedWallGlyph(x, y int, tile domain.Tile, dist float64) rune {
	if tile == domain.TileDoor {
		if (x+y)%2 == 0 {
			return '+'
		}
		return '|'
	}
	if dist > 10 {
		return '.'
	}
	if (x/2+y)%3 == 0 {
		return '#'
	}
	return '%'
}

func wallStyle(tile domain.Tile, dist float64) tcell.Style {
	color := tcell.ColorGray
	if tile == domain.TileDoor {
		color = tcell.ColorOrange
	} else if dist > 9 {
		color = tcell.ColorDarkSlateGray
	} else if dist > 5 {
		color = tcell.ColorDimGray
	}
	return tcell.StyleDefault.Foreground(color).Background(tcell.ColorBlack)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func enemyGlyph(kind domain.EnemyKind) rune {
	switch kind {
	case domain.EnemyZombie:
		return 'z'
	case domain.EnemyVampire:
		return 'v'
	case domain.EnemyGhost:
		return 'g'
	case domain.EnemyOgre:
		return 'O'
	case domain.EnemySnake:
		return 's'
	case domain.EnemyMimic:
		return 'm'
	default:
		return 'M'
	}
}

func enemyStyle(kind domain.EnemyKind) tcell.Style {
	switch kind {
	case domain.EnemyZombie:
		return tcell.StyleDefault.Foreground(tcell.ColorGreen)
	case domain.EnemyVampire:
		return tcell.StyleDefault.Foreground(tcell.ColorRed)
	case domain.EnemyGhost:
		return tcell.StyleDefault.Foreground(tcell.ColorWhite)
	case domain.EnemyOgre:
		return tcell.StyleDefault.Foreground(tcell.ColorYellow)
	case domain.EnemySnake:
		return tcell.StyleDefault.Foreground(tcell.ColorWhite)
	case domain.EnemyMimic:
		return tcell.StyleDefault.Foreground(tcell.ColorWhite)
	default:
		return tcell.StyleDefault.Foreground(tcell.ColorRed)
	}
}

func describeItem(item domain.Item) string {
	parts := []string{string(item.Kind)}
	if item.SubType != "" {
		parts = append(parts, item.SubType)
	}
	if item.Health != 0 {
		parts = append(parts, fmt.Sprintf("HP %+d", item.Health))
	}
	if item.MaxHealth != 0 {
		parts = append(parts, fmt.Sprintf("MaxHP %+d", item.MaxHealth))
	}
	if item.Agility != 0 {
		parts = append(parts, fmt.Sprintf("AGI %+d", item.Agility))
	}
	if item.Strength != 0 {
		parts = append(parts, fmt.Sprintf("STR %+d", item.Strength))
	}
	if item.Duration != 0 {
		parts = append(parts, fmt.Sprintf("%d ходов", item.Duration))
	}
	return strings.Join(parts, " ")
}

func pad(s string, width int) string {
	if len([]rune(s)) >= width {
		r := []rune(s)
		return string(r[:width])
	}
	return s + strings.Repeat(" ", width-len([]rune(s)))
}
