package storage

import (
	"encoding/json"
	"path/filepath"

	"github.com/nutsdb/nutsdb"
)

type Store struct {
	db *nutsdb.DB
}

type StoreInstance[T any] struct {
	*Store
	table string
}

func New(storage_path string) (*Store, error) {
	db, err := nutsdb.Open(nutsdb.DefaultOptions, nutsdb.WithDir(filepath.Join(storage_path, "storage")))
	if err != nil {
		return nil, err
	}

	store := &Store{
		db: db,
	}

	return store, nil
}

func GetInstance[T any](s *Store, table string) (*StoreInstance[T], error) {
	err := s.db.Update(func(tx *nutsdb.Tx) error {
		if !tx.ExistBucket(nutsdb.DataStructureBTree, table) {
			return tx.NewBucket(nutsdb.DataStructureBTree, table)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &StoreInstance[T]{
		Store: s,
		table: table,
	}, nil
}

func (s *StoreInstance[T]) Close() error {
	return s.db.Close()
}

func (s *StoreInstance[T]) Set(key string, value *T) error {
	return s.db.Update(func(tx *nutsdb.Tx) error {
		object, err := json.Marshal(value)
		if err != nil {
			return err
		}

		return tx.Put(s.table, []byte(key), object, 0)
	})
}

func (s *StoreInstance[T]) Get(key string) (*T, error) {
	var result *T
	err := s.db.View(func(tx *nutsdb.Tx) error {
		data, err := tx.Get(s.table, []byte(key))

		if err != nil {
			return err
		}

		if len(data) < 1 {
			return nil
		}

		return json.Unmarshal(data, &result)
	})
	return result, err
}

func (s *StoreInstance[T]) GetAll() ([]*T, error) {
	var result []*T
	err := s.db.View(func(tx *nutsdb.Tx) error {
		_, entries, err := tx.GetAll(s.table)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			var data *T
			if err := json.Unmarshal(entry, &data); err != nil {
				return err
			}

			result = append(result, data)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *StoreInstance[T]) Del(key string) error {
	return s.db.Update(func(tx *nutsdb.Tx) error {
		return tx.Delete(s.table, []byte(key))
	})
}

func (s *StoreInstance[T]) DelAll() error {
	return s.db.Update(func(tx *nutsdb.Tx) error {
		err := tx.DeleteBucket(nutsdb.DataStructureBTree, s.table)
		if err != nil {
			return err
		}

		return tx.NewBucket(nutsdb.DataStructureBTree, s.table)
	})
}
