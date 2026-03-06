package asset

import "github.com/ttpreport/ligolo-mp/v2/internal/storage"

type AssetRepository struct {
	storage *storage.StoreInstance[Asset]
}

var table = "assets"

func NewAssetRepository(store *storage.Store) (*AssetRepository, error) {
	storeInstance, err := storage.GetInstance[Asset](store, table)
	if err != nil {
		return nil, err
	}

	return &AssetRepository{
		storage: storeInstance,
	}, nil
}

func (repo *AssetRepository) GetOne(name string) (*Asset, error) {
	return repo.storage.Get(name)
}

func (repo *AssetRepository) GetAll() ([]*Asset, error) {
	return repo.storage.GetAll()
}

func (repo *AssetRepository) Save(asset *Asset) error {
	return repo.storage.Set(asset.Name, asset)
}

func (repo *AssetRepository) Remove(key string) error {
	return repo.storage.Del(key)
}
