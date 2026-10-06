package datalayer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"rogue/internal/domain"
)

func TestRecordsAreSortedByTreasure(t *testing.T) {
	store := NewStore(t.TempDir())
	if err := store.AppendRecord(domain.RunRecord{Stats: domain.RunStats{Treasure: 10, ReachedLevel: 3}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRecord(domain.RunRecord{Stats: domain.RunStats{Treasure: 30, ReachedLevel: 1}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendRecord(domain.RunRecord{Stats: domain.RunStats{Treasure: 30, ReachedLevel: 5}}); err != nil {
		t.Fatal(err)
	}
	records, err := store.Records()
	if err != nil {
		t.Fatal(err)
	}
	if records[0].Stats.Treasure != 30 || records[0].Stats.ReachedLevel != 5 {
		t.Fatalf("first record = %+v, want highest treasure and deeper level", records[0].Stats)
	}
	if records[2].Stats.Treasure != 10 {
		t.Fatalf("last treasure = %d, want 10", records[2].Stats.Treasure)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	game := domain.NewGame(123)
	game.Stats.Treasure = 77
	if err := store.SaveGame(game.Snapshot()); err != nil {
		t.Fatal(err)
	}
	state, err := store.LoadGame()
	if err != nil {
		t.Fatal(err)
	}
	loaded := domain.LoadFromSnapshot(state)
	if loaded.Seed != 123 || loaded.Stats.Treasure != 77 {
		t.Fatalf("loaded game = seed %d treasure %d", loaded.Seed, loaded.Stats.Treasure)
	}
	if !reflect.DeepEqual(game.Snapshot(), loaded.Snapshot()) {
		t.Fatal("save/load changed game state")
	}
	game.Stats.Treasure = 88
	if err := store.SaveGame(game.Snapshot()); err != nil {
		t.Fatal(err)
	}
	state, err = store.LoadGame()
	if err != nil || state.Game.Stats.Treasure != 88 {
		t.Fatalf("overwrite failed: %v", err)
	}
}

func TestLoadRejectsBrokenSave(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, data := range []string{`{`, `{}`, `null`} {
		if err := os.WriteFile(filepath.Join(store.Dir, "save.json"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.LoadGame(); err == nil {
			t.Fatalf("accepted invalid save: %s", data)
		}
	}
	state := domain.NewGame(42).Snapshot()
	state.Game.Level.Tiles[0] = nil
	if err := store.SaveGame(state); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadGame(); err == nil {
		t.Fatal("accepted broken map")
	}
}
