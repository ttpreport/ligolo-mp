package operator

import (
	"github.com/ttpreport/ligolo-mp/internal/storage"
)

type OperatorRepository struct {
	storage *storage.StoreInstance[Operator]
}

var table = "operators"

func NewOperatorRepository(store *storage.Store) (*OperatorRepository, error) {
	storeInstance, err := storage.GetInstance[Operator](store, table)
	if err != nil {
		return nil, err
	}

	return &OperatorRepository{
		storage: storeInstance,
	}, nil
}

func (repo *OperatorRepository) GetOne(name string) (*Operator, error) {
	result, err := repo.storage.Get(name)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (repo *OperatorRepository) GetAll() ([]*Operator, error) {
	return repo.storage.GetAll()
}

func (repo *OperatorRepository) Save(oper *Operator) error {
	return repo.storage.Set(oper.Name, oper)
}

func (repo *OperatorRepository) Remove(oper *Operator) error {
	return repo.storage.Del(oper.Name)
}

func (repo *OperatorRepository) Exists(oper *Operator) bool {
	dbOper, _ := repo.storage.Get(oper.Name)
	if dbOper == nil {
		return false
	} else {
		return true
	}
}
