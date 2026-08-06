package main

import (
	"fmt"
	"log"
	"os"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/db/migrations"
	"universal-curriculum/internal/server"
)

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}

	cfg, err := server.LoadConfig()
	if err != nil {
		log.Fatalf("Load configuration: %v", err)
	}

	database, err := db.Open(cfg.PostgresConnString())
	if err != nil {
		log.Fatalf("Open database: %v", err)
	}
	defer database.Close()

	switch os.Args[1] {
	case "up":
		err = migrations.Up(database)
	case "status":
		err = migrations.Status(database)
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatalf("Run migration command %q: %v", os.Args[1], err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: migrate <up|status>")
}
