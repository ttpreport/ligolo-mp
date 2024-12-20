package crl

import (
	"github.com/ttpreport/ligolo-mp/internal/storage"
)

type CRLRepository struct {
	storage *storage.StoreInstance[RevokedCertificate]
}

var table = "crl"

func NewCRLRepository(store *storage.Store) (*CRLRepository, error) {
	storeInstance, err := storage.GetInstance[RevokedCertificate](store, table)
	if err != nil {
		return nil, err
	}

	return &CRLRepository{
		storage: storeInstance,
	}, nil
}

func (repo *CRLRepository) GetOne(hash string) *RevokedCertificate {
	result, err := repo.storage.Get(hash)
	if err != nil {
		return nil
	}

	return result
}

func (repo *CRLRepository) GetAll() ([]*RevokedCertificate, error) {
	return repo.storage.GetAll()
}

func (repo *CRLRepository) Save(cert *RevokedCertificate) error {
	return repo.storage.Set(cert.Hash(), cert)
}

func (repo *CRLRepository) Remove(key string) error {
	return repo.storage.Del(key)
}
