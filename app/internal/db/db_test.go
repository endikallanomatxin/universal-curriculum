package db

import (
	"database/sql"
	"testing"
)

func TestConfigureDatabasePoolBoundsConnections(t *testing.T) {
	database, err := sql.Open("postgres", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	configureDatabasePool(database)
	if got := database.Stats().MaxOpenConnections; got != databaseMaxOpenConnections {
		t.Fatalf("maximum open connections = %d, want %d", got, databaseMaxOpenConnections)
	}
}
