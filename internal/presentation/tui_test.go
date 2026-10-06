package presentation

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"rogue/internal/datalayer"
	"rogue/internal/domain"
)

func TestCyrillicTextUsesAdjacentCells(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	drawText(screen, 0, 0, tcell.StyleDefault, "Игра")
	for x, want := range []rune("Игра") {
		got, _, _, _ := screen.GetContent(x, 0)
		if got != want {
			t.Fatalf("cell %d = %q; want %q", x, got, want)
		}
	}
}

func TestGameLoopDrawAndSave(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 35)
	store := datalayer.NewStore(t.TempDir())
	game := domain.NewGame(42)
	app := NewApp(game, store)
	app.screen = screen
	app.drawGame()
	ch, _, _, _ := screen.GetContent(game.Player.Pos.X, game.Player.Pos.Y)
	if ch != '@' {
		t.Fatalf("player glyph = %q", ch)
	}
	for _, key := range []rune{'v', 'p', 'q'} {
		if err := screen.PostEvent(tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone)); err != nil {
			t.Fatal(err)
		}
	}
	if err := app.gameLoop(); err != nil {
		t.Fatal(err)
	}
	state, err := store.LoadGame()
	if err != nil {
		t.Fatal(err)
	}
	if state.Game.Seed != 42 || !app.view3D {
		t.Fatal("view toggle or save failed")
	}
}
