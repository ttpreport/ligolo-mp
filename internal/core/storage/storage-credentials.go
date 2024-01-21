package storage

import "context"

type credsRow struct {
	Id           int
	Name         string
	Server       string
	CaCert       []byte
	OperatorCert []byte
	OperatorKey  []byte
}

func (storage *Store) GetCreds() ([]credsRow, error) {
	var err error
	var ret []credsRow

	rows, err := storage.db.QueryContext(
		context.Background(),
		`SELECT id, name, server, ca_cert, operator_cert, operator_key FROM credentials;`,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var cred credsRow

		if err := rows.Scan(
			&cred.Id, &cred.Name, &cred.Server, &cred.CaCert, &cred.OperatorCert, &cred.OperatorKey,
		); err != nil {
			return nil, err
		}
		ret = append(ret, cred)
	}
	return ret, err
}

func (storage *Store) AddCred(name string, server string, cacert []byte, operatorcert []byte, operatorkey []byte) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`INSERT INTO credentials (name, server, ca_cert, operator_cert, operator_key) VALUES (?,?,?,?,?);`, name, server, cacert, operatorcert, operatorkey,
	)

	if err != nil {
		return err
	}

	return err
}

func (storage *Store) DelCred(id int) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`DELETE FROM credentials WHERE id = ?;`, id,
	)

	if err != nil {
		return err
	}

	return nil
}
