package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func configure(db *sql.DB) error {
	pragmas := []string{
		"busy_timeout = 5000",
		"journal_mode = WAL",
		"synchronous = NORMAL",
		"cache_size = 1000000000", // 1GB
		"foreign_keys = true",
		"temp_store = memory",
		"mmap_size = 3000000000",
	}

	for _, pragma := range pragmas {
		_, err := db.Exec("PRAGMA " + pragma)
		if err != nil {
			return err
		}
	}
	return nil
}

func Connect(ctx context.Context, dbname string) (*Queries, error) {
	db, err := sql.Open("sqlite", dbname)
	if err != nil {
		return nil, err
	}

	if err := configure(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to configure database: %w", err)
	}

	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database schema: %w", err)
	}

	return New(db), nil
}
