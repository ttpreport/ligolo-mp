package storage

import (
	"context"
	"database/sql"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(storage_path string) (*Store, error) {
	db, err := sql.Open("sqlite", filepath.Join(storage_path, "storage.db"))

	if err != nil {
		return nil, err
	}

	ret := &Store{
		db: db,
	}

	return ret, nil
}

func (storage *Store) InitServer() error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS tuns (
			alias TEXT NOT NULL, 
			name TEXT NOT NULL,
			is_loopback BOOLEAN NOT NULL CHECK (is_loopback IN (0, 1)),
			UNIQUE(alias, name)
		)`,
	)

	if err != nil {
		return err
	}

	_, err = storage.db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS agent_cache (
			tun_alias INTEGER NOT NULL UNIQUE,
			agent_hash TEXT NOT NULL
		)`,
	)

	if err != nil {
		return err
	}

	_, err = storage.db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS certs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			certificate TEXT NOT NULL,
			key TEXT NOT NULL
		)`,
	)

	if err != nil {
		return err
	}

	_, err = storage.db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS operators (
			name TEXT NOT NULL UNIQUE,
			cert_id INTEGER NOT NULL
		)`,
	)

	if err != nil {
		return err
	}

	return nil
}

func (storage *Store) InitClient() error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`CREATE TABLE IF NOT EXISTS credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL, 
			server TEXT NOT NULL,
			ca_cert TEXT NOT NULL,
			operator_cert TEXT NOT NULL,
			operator_key TEXT NOT NULL
		)`,
	)

	if err != nil {
		return err
	}

	return nil
}
