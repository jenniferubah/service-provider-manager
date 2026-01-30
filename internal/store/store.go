package store

import (
	store "github.com/dcm-project/service-provider-manager/internal/store/resource_manager"
	"gorm.io/gorm"
)

type Store interface {
	Close() error
	Provider() Provider
	ServiceTypeInstance() store.ServiceTypeInstance
}

type DataStore struct {
	db       *gorm.DB
	provider Provider
	instance store.ServiceTypeInstance
}

func NewStore(db *gorm.DB) Store {
	return &DataStore{
		db:       db,
		provider: NewProvider(db),
		instance: store.NewServiceTypeInstance(db),
	}
}

func (s *DataStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *DataStore) Provider() Provider {
	return s.provider
}

func (s *DataStore) ServiceTypeInstance() store.ServiceTypeInstance {
	return s.instance
}
