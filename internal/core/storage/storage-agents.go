package storage

import "context"

type agentsRow struct {
	TunAlias string
}

func (storage *Store) GetAgentTun(agent_hash string) (*agentsRow, error) {
	var err error
	ret := &agentsRow{}

	row := storage.db.QueryRowContext(
		context.Background(),
		`SELECT tun_alias FROM agent_cache WHERE agent_hash = ?;`, agent_hash,
	)

	if err = row.Scan(&ret.TunAlias); err != nil {
		return nil, nil
	}

	return ret, err
}

func (storage *Store) AddRelay(tun_alias string, agent_hash string) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`REPLACE INTO agent_cache (tun_alias, agent_hash) VALUES (?,?);`, tun_alias, agent_hash,
	)

	if err != nil {
		return err
	}

	return nil
}

func (storage *Store) DelRelay(agent_hash string) error {
	var err error
	_, err = storage.db.ExecContext(
		context.Background(),
		`DELETE FROM agent_cache WHERE agent_hash = ?;`, agent_hash,
	)

	if err != nil {
		return err
	}

	return nil
}
