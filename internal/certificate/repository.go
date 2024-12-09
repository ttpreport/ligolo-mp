package certificate

import (
	"github.com/ttpreport/ligolo-mp/internal/storage"
)

type CertificateRepository struct {
	storage *storage.StoreInstance[Certificate]
}

var table = "certificates"

func NewCertificateRepository(store *storage.Store) (*CertificateRepository, error) {
	storeInstance, err := storage.GetInstance[Certificate](store, table)
	if err != nil {
		return nil, err
	}

	return &CertificateRepository{
		storage: storeInstance,
	}, nil
}

func (repo *CertificateRepository) GetOne(name string) *Certificate {
	result, err := repo.storage.Get(name)
	if err != nil {
		return nil
	}

	return result
}

func (repo *CertificateRepository) GetAll() ([]*Certificate, error) {
	return repo.storage.GetAll()
}

func (repo *CertificateRepository) Save(cert *Certificate) error {
	return repo.storage.Set(cert.Name, cert)
}

func (repo *CertificateRepository) Remove(key string) error {
	return repo.storage.Del(key)
}
