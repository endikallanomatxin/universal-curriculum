package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
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
	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return database, nil
}
