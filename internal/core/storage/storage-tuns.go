package storage

import "context"

type tunsRow struct {
	Alias      string
	Name       string
	IsLoopback bool
}

func (storage *Store) GetTuns() ([]tunsRow, error) {
	var err error
	var ret []tunsRow

	rows, err := storage.db.QueryContext(
		context.Background(),
		`SELECT alias, name, is_loopback FROM tuns;`,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var tun tunsRow

		if err := rows.Scan(
			&tun.Alias, &tun.Name, &tun.IsLoopback,
		); err != nil {
			return nil, err
		}
		ret = append(ret, tun)
	}
	return ret, err
}

func (storage *Store) AddTun(alias string, name string, isLoopback bool) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`INSERT INTO tuns (alias, name, is_loopback) VALUES (?,?,?);`, alias, name, isLoopback,
	)

	if err != nil {
		return err
	}

	return err
}

func (storage *Store) DelTun(alias string) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`DELETE FROM tuns WHERE alias = ?;`, alias,
	)

	if err != nil {
		return err
	}

	_, err = storage.db.ExecContext(
		context.Background(),
		`DELETE FROM routes WHERE tun_alias = ?;`, alias,
	)

	if err != nil {
		return err
	}

	return nil
}
