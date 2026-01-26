package store

import "gorm.io/gorm"

type Store interface {
	Close() error
	Provider() Provider
}

type DataStore struct {
	db       *gorm.DB
	provider Provider
}

func NewStore(db *gorm.DB) Store {
	return &DataStore{
		db:       db,
		provider: NewProvider(db),
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

