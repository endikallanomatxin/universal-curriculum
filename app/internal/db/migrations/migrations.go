package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed sql
var fs embed.FS

func Up(database *sql.DB) error {
	if err := setup(); err != nil {
		return err
	}
	return goose.Up(database, "sql")
}

func Status(database *sql.DB) error {
	if err := setup(); err != nil {
		return err
	}
	return goose.Status(database, "sql")
}

func setup() error {
	goose.SetBaseFS(fs)
	return goose.SetDialect("postgres")
}
