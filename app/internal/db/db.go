package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

const (
	databaseMaxOpenConnections = 12
	databaseMaxIdleConnections = 4
)

type QueryExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func Open(connectionString string) (*sql.DB, error) {
	database, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	configureDatabasePool(database)
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return database, nil
}

func configureDatabasePool(database *sql.DB) {
	database.SetMaxOpenConns(databaseMaxOpenConnections)
	database.SetMaxIdleConns(databaseMaxIdleConnections)
	database.SetConnMaxIdleTime(5 * time.Minute)
	database.SetConnMaxLifetime(30 * time.Minute)
}
