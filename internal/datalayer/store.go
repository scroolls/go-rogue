package datalayer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"rogue/internal/domain"
)

type Store struct {
	Dir string
}

func NewStore(dir string) Store {
	return Store{Dir: dir}
}

func (s Store) SaveGame(state domain.SaveState) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(s.Dir, "save.json"), data)
}

func (s Store) LoadGame() (domain.SaveState, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, "save.json"))
	if err != nil {
		return domain.SaveState{}, err
	}
	var state domain.SaveState
	if err := json.Unmarshal(data, &state); err != nil {
		return domain.SaveState{}, err
	}
	if err := validateSave(state); err != nil {
		return domain.SaveState{}, err
	}
	return state, nil
}

func (s Store) AppendRecord(record domain.RunRecord) error {
	if record.Finished.IsZero() {
		record.Finished = time.Now()
	}
	records, err := s.Records()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	records = append(records, record)
	sortRecords(records)
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	return writeJSONFile(filepath.Join(s.Dir, "scoreboard.json"), data)
}

// Write the complete file before replacing the previous save.
func writeJSONFile(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".save-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func validateSave(state domain.SaveState) error {
	g := state.Game
	l := g.Level
	if l.Width != domain.BoardWidth || l.Height != domain.BoardHeight || l.Depth < 1 || l.Depth > domain.MaxDepth {
		return fmt.Errorf("invalid saved level dimensions or depth")
	}
	if len(l.Tiles) != l.Height || len(l.Explored) != l.Height {
		return fmt.Errorf("invalid saved map")
	}
	for y := 0; y < l.Height; y++ {
		if len(l.Tiles[y]) != l.Width || len(l.Explored[y]) != l.Width {
			return fmt.Errorf("invalid saved map row")
		}
	}
	inBounds := func(p domain.Point) bool { return p.X >= 0 && p.X < l.Width && p.Y >= 0 && p.Y < l.Height }
	if !inBounds(g.Player.Pos) {
		return fmt.Errorf("invalid saved player position")
	}
	if len(l.Rooms) != 9 || l.StartRoom < 0 || l.StartRoom >= len(l.Rooms) || l.ExitRoom < 0 || l.ExitRoom >= len(l.Rooms) {
		return fmt.Errorf("invalid saved rooms")
	}
	for _, room := range l.Rooms {
		if room.W < 3 || room.H < 3 || room.X < 0 || room.Y < 0 || room.W > l.Width || room.H > l.Height || room.X > l.Width-room.W || room.Y > l.Height-room.H || !inBounds(room.Center) {
			return fmt.Errorf("invalid saved room bounds")
		}
	}
	for _, enemy := range l.Enemies {
		if !inBounds(enemy.Pos) {
			return fmt.Errorf("invalid saved enemy position")
		}
	}
	return nil
}

func (s Store) Records() ([]domain.RunRecord, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, "scoreboard.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []domain.RunRecord{}, nil
		}
		return nil, err
	}
	var records []domain.RunRecord
	if len(data) == 0 {
		return []domain.RunRecord{}, nil
	}
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, err
	}
	sortRecords(records)
	return records, nil
}

func sortRecords(records []domain.RunRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Stats.Treasure == records[j].Stats.Treasure {
			return records[i].Stats.ReachedLevel > records[j].Stats.ReachedLevel
		}
		return records[i].Stats.Treasure > records[j].Stats.Treasure
	})
}
