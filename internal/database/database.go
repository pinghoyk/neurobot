package database

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть базу данных: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.applySchema(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("не удалось применить схему: %w", err)
	}

	return db, nil
}

// applySchema читает и выполняет schema.sql
func (db *DB) applySchema() error {
	_, err := db.conn.Exec(schemaSQL)
	return err
}

func (db *DB) Close() error {
	return db.conn.Close()
}