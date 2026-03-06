package storage

import (
	"encoding/json"
	"path/filepath"

	"database/sql"

	_ "modernc.org/sqlite"
)

// Store wraps a single *sql.DB. Individual ops are safe without a mutex:
// *sql.DB is goroutine-safe, and WAL+MaxOpenConns(1) serialises writers at
// the SQLite level. The per-instance StoreInstance.mu was removed as redundant.
type Store struct {
	db *sql.DB
}

// StoreInstance is a typed view of one table inside a Store.
type StoreInstance[T any] struct {
	*Store
	table string
}

type StorageRow struct {
	Name  string
	Value []byte
}

func New(storage_path string) (*Store, error) {
	db, err := sql.Open("sqlite", filepath.Join(storage_path, "data.db"))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{
		db: db,
	}, nil
}

func (store *Store) Close() error {
	return store.db.Close()
}

func GetInstance[T any](s *Store, table string) (*StoreInstance[T], error) {
	query := "CREATE TABLE IF NOT EXISTS " + table + " (name TEXT NOT NULL PRIMARY KEY, value BLOB)"
	_, err := s.db.Exec(query)
	if err != nil {
		return nil, err
	}

	return &StoreInstance[T]{
		Store: s,
		table: table,
	}, nil
}

func (s *StoreInstance[T]) Set(key string, value *T) error {
	object, err := json.Marshal(value)
	if err != nil {
		return err
	}

	query := "INSERT INTO " + s.table + " (name, value) VALUES(?, ?) ON CONFLICT (name) DO UPDATE SET value = ?"
	_, err = s.db.Exec(query, key, object, object)
	return err
}

func (s *StoreInstance[T]) Get(key string) (*T, error) {
	var result *T
	row := new(StorageRow)
	query := "SELECT name, value FROM " + s.table + " WHERE name = ?"
	err := s.db.QueryRow(query, key).Scan(&row.Name, &row.Value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		} else {
			return nil, err
		}
	}

	err = json.Unmarshal(row.Value, &result)
	return result, err
}

func (s *StoreInstance[T]) GetAll() ([]*T, error) {
	var result []*T

	query := "SELECT value FROM " + s.table
	rows, err := s.db.Query(query)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, nil
		} else {
			return nil, err
		}
	}

	for rows.Next() {
		row := new(StorageRow)
		rows.Scan(&row.Value)

		var value *T
		if err := json.Unmarshal(row.Value, &value); err != nil {
			return nil, err
		}

		result = append(result, value)
	}

	return result, err
}

func (s *StoreInstance[T]) Del(key string) error {
	query := "DELETE FROM " + s.table + " WHERE name = ?"
	_, err := s.db.Exec(query, key)
	return err
}

func (s *StoreInstance[T]) DelAll() error {
	query := "DELETE FROM " + s.table
	_, err := s.db.Exec(query)
	return err
}

