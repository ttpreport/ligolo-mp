package storage

import (
	"context"
	"database/sql"
)

type operatorsRow struct {
	Name   string
	CertId int64
}

func (storage *Store) GetOperators() ([]operatorsRow, error) {
	var err error
	var ret []operatorsRow

	rows, err := storage.db.QueryContext(
		context.Background(),
		`SELECT name, cert_id FROM operators;`,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var operator operatorsRow

		if err := rows.Scan(
			&operator.Name, &operator.CertId,
		); err != nil {
			return nil, err
		}
		ret = append(ret, operator)
	}
	return ret, err
}

func (storage *Store) GetOperator(name string) (*operatorsRow, error) {
	ret := &operatorsRow{}

	err := storage.db.QueryRowContext(
		context.Background(),
		`SELECT name, cert_id FROM operators WHERE name = ?;`, name,
	).Scan(
		&ret.Name, &ret.CertId,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return ret, nil
}

func (storage *Store) AddOperator(name string, certId int64) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`INSERT INTO operators (name, cert_id) VALUES (?,?);`, name, certId,
	)

	if err != nil {
		return err
	}

	return nil
}

func (storage *Store) DelOperator(name string) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`DELETE FROM operators WHERE name = ?;`, name,
	)

	if err != nil {
		return err
	}

	return nil
}
