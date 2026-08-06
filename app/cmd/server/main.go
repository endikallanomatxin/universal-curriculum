package main

import (
	"log"

	"universal-curriculum/internal/server"
)

func main() {
	app, err := server.Setup()
	if err != nil {
		log.Fatalf("Set up server: %v", err)
	}
	if err := app.Run(); err != nil {
		log.Fatalf("Run server: %v", err)
	}
}
