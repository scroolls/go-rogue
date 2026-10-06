package main

import (
	"log"
	"time"

	"rogue/internal/datalayer"
	"rogue/internal/domain"
	"rogue/internal/presentation"
)

func main() {
	store := datalayer.NewStore("data")
	game := domain.NewGame(time.Now().UnixNano())
	app := presentation.NewApp(game, store)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
