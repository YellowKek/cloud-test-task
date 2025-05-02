package db

import (
	"database/sql"
	"fmt"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/stdlib"
)

func NewDB(uri string) (*sql.DB, error) {
	connCfg, err := pgx.ParseURI(uri)
	if err != nil {
		return nil, fmt.Errorf("pgx parse config error: %v", err)
	}

	db := stdlib.OpenDB(connCfg)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db error: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS client_configs (
			client_id TEXT PRIMARY KEY,
			capacity INTEGER NOT NULL,
			refill_interval BIGINT NOT NULL
		)
	`)
	if err != nil {
		return nil, err
	}

	return db, nil
}
