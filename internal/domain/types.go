package domain

import "time"

const (
	BoardWidth  = 80
	BoardHeight = 26
	MaxDepth    = 21
)

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Tile int

const (
	TileWall Tile = iota
	TileFloor
	TileCorridor
	TileExit
	TileDoor
)

type Room struct {
	ID      int   `json:"id"`
	X       int   `json:"x"`
	Y       int   `json:"y"`
	W       int   `json:"w"`
	H       int   `json:"h"`
	Center  Point `json:"center"`
	Started bool  `json:"started"`
	Exit    bool  `json:"exit"`
}

func (r Room) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.W && p.Y >= r.Y && p.Y < r.Y+r.H
}

type EnemyKind string

const (
	EnemyZombie  EnemyKind = "zombie"
	EnemyVampire EnemyKind = "vampire"
	EnemyGhost   EnemyKind = "ghost"
	EnemyOgre    EnemyKind = "ogre"
	EnemySnake   EnemyKind = "snake_mage"
	EnemyMimic   EnemyKind = "mimic"
)

type ItemKind string

const (
	ItemTreasure ItemKind = "treasure"
	ItemFood     ItemKind = "food"
	ItemElixir   ItemKind = "elixir"
	ItemScroll   ItemKind = "scroll"
	ItemWeapon   ItemKind = "weapon"
	ItemKey      ItemKind = "key"
)

type DoorColor string

const (
	DoorRed    DoorColor = "red"
	DoorBlue   DoorColor = "blue"
	DoorYellow DoorColor = "yellow"
)

type StatKind string

const (
	StatHealth    StatKind = "health"
	StatMaxHealth StatKind = "max_health"
	StatAgility   StatKind = "agility"
	StatStrength  StatKind = "strength"
)

type Item struct {
	Kind      ItemKind `json:"kind"`
	SubType   string   `json:"subtype"`
	Health    int      `json:"health"`
	MaxHealth int      `json:"max_health"`
	Agility   int      `json:"agility"`
	Strength  int      `json:"strength"`
	Value     int      `json:"value"`
	Duration  int      `json:"duration"`
}

type GroundItem struct {
	Item Item  `json:"item"`
	Pos  Point `json:"pos"`
}

type Door struct {
	Pos    Point     `json:"pos"`
	Color  DoorColor `json:"color"`
	Locked bool      `json:"locked"`
}

type Backpack struct {
	Items map[ItemKind][]Item `json:"items"`
	Keys  map[DoorColor]bool  `json:"keys"`
}

func NewBackpack() Backpack {
	return Backpack{Items: map[ItemKind][]Item{
		ItemFood: {}, ItemElixir: {}, ItemScroll: {}, ItemWeapon: {}, ItemKey: {},
	}, Keys: map[DoorColor]bool{}}
}

func (b *Backpack) Add(item Item) bool {
	if b.Items == nil {
		*b = NewBackpack()
	}
	if item.Kind == ItemTreasure {
		return true
	}
	if item.Kind == ItemKey {
		b.Keys[DoorColor(item.SubType)] = true
		return true
	}
	list := b.Items[item.Kind]
	if len(list) >= 9 {
		return false
	}
	b.Items[item.Kind] = append(list, item)
	return true
}

func (b *Backpack) Remove(kind ItemKind, index int) (Item, bool) {
	list := b.Items[kind]
	if index < 0 || index >= len(list) {
		return Item{}, false
	}
	item := list[index]
	b.Items[kind] = append(list[:index], list[index+1:]...)
	return item, true
}

type TimedEffect struct {
	Stat  StatKind `json:"stat"`
	Value int      `json:"value"`
	Turns int      `json:"turns"`
}

type Character struct {
	MaxHealth int           `json:"max_health"`
	Health    int           `json:"health"`
	Agility   int           `json:"agility"`
	Strength  int           `json:"strength"`
	Weapon    *Item         `json:"weapon,omitempty"`
	Pos       Point         `json:"pos"`
	Bag       Backpack      `json:"bag"`
	Effects   []TimedEffect `json:"effects"`
}

type Enemy struct {
	ID          int       `json:"id"`
	Kind        EnemyKind `json:"kind"`
	Health      int       `json:"health"`
	MaxHealth   int       `json:"max_health"`
	Agility     int       `json:"agility"`
	Strength    int       `json:"strength"`
	Hostility   int       `json:"hostility"`
	Pos         Point     `json:"pos"`
	RoomID      int       `json:"room_id"`
	Asleep      bool      `json:"asleep"`
	Resting     bool      `json:"resting"`
	FirstMiss   bool      `json:"first_miss"`
	Invisible   bool      `json:"invisible"`
	PatternTick int       `json:"pattern_tick"`
}

type Level struct {
	Depth     int          `json:"depth"`
	Width     int          `json:"width"`
	Height    int          `json:"height"`
	Tiles     [][]Tile     `json:"tiles"`
	Explored  [][]bool     `json:"explored"`
	Rooms     []Room       `json:"rooms"`
	Corridors []Point      `json:"corridors"`
	Doors     []Door       `json:"doors"`
	Enemies   []Enemy      `json:"enemies"`
	Items     []GroundItem `json:"items"`
	StartRoom int          `json:"start_room"`
	ExitRoom  int          `json:"exit_room"`
}

type RunStats struct {
	Treasure     int `json:"treasure"`
	ReachedLevel int `json:"reached_level"`
	Defeated     int `json:"defeated"`
	FoodEaten    int `json:"food_eaten"`
	ElixirsDrunk int `json:"elixirs_drunk"`
	ScrollsRead  int `json:"scrolls_read"`
	HitsDealt    int `json:"hits_dealt"`
	HitsTaken    int `json:"hits_taken"`
	CellsWalked  int `json:"cells_walked"`
}

type RunRecord struct {
	Stats    RunStats  `json:"stats"`
	Won      bool      `json:"won"`
	Finished time.Time `json:"finished"`
}

type GameState string

const (
	StatePlaying GameState = "playing"
	StateDead    GameState = "dead"
	StateWon     GameState = "won"
)

type Game struct {
	Seed        int64     `json:"seed"`
	Turn        int       `json:"turn"`
	State       GameState `json:"state"`
	Player      Character `json:"player"`
	Level       Level     `json:"level"`
	Stats       RunStats  `json:"stats"`
	Message     string    `json:"message"`
	SleepTurns  int       `json:"sleep_turns"`
	Balance     int       `json:"balance"`
	nextEnemyID int       `json:"-"`
}

type SaveState struct {
	Game Game `json:"game"`
}

type EventResult struct {
	Message string `json:"message"`
	Done    bool   `json:"done"`
}
