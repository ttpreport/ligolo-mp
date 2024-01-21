package operator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Credentials struct {
	Name         string
	Server       string
	CACert       []byte
	OperatorCert []byte
	OperatorKey  []byte
}

func SaveToFile(path string, name string, server string, CACert []byte, OperatorCert []byte, OperatorKey []byte) (string, error) {
	operatorConfig := Credentials{
		Name:         name,
		Server:       server,
		CACert:       CACert,
		OperatorCert: OperatorCert,
		OperatorKey:  OperatorKey,
	}

	operatorConfigBytes, err := json.Marshal(operatorConfig)
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(path, fmt.Sprintf("%s_%s_ligolo-mp.json", name, server))
	if err = os.WriteFile(fullPath, operatorConfigBytes, os.ModePerm); err != nil {
		return "", err
	}

	return fullPath, nil
}

func LoadFromFile(path string) (*Credentials, error) {
	cfg := &Credentials{}

	cfgBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(cfgBytes, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
