package storage

import (
	"context"
)

type certsRow struct {
	Id          int64
	Name        string
	Certificate []byte
	Key         []byte
}

func (storage *Store) GetCerts() ([]certsRow, error) {
	var err error
	var ret []certsRow

	rows, err := storage.db.QueryContext(
		context.Background(),
		`SELECT id, name, certificate, key FROM certs;`,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var cert certsRow

		if err := rows.Scan(
			&cert.Id, &cert.Name, &cert.Certificate, &cert.Key,
		); err != nil {
			return nil, err
		}
		ret = append(ret, cert)
	}
	return ret, err
}

func (storage *Store) GetCert(name string) (*certsRow, error) {
	var err error
	ret := &certsRow{}

	row := storage.db.QueryRowContext(
		context.Background(),
		`SELECT id, name, certificate, key FROM certs WHERE name = ?;`, name,
	)

	if err != nil {
		return nil, err
	}

	if err := row.Scan(
		&ret.Id, &ret.Name, &ret.Certificate, &ret.Key,
	); err != nil {
		return nil, nil
	}

	return ret, err
}

func (storage *Store) AddCert(name string, certificate []byte, key []byte) (int64, error) {
	var err error
	result, err := storage.db.ExecContext(
		context.Background(),
		`INSERT INTO certs (name, certificate, key) VALUES (?,?,?);`, name, certificate, key,
	)

	if err != nil {
		return -1, err
	}

	return result.LastInsertId()
}

func (storage *Store) DelCert(name string) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`DELETE FROM certs WHERE name = ?;`, name,
	)

	if err != nil {
		return err
	}

	return nil
}
