package store

import (
	"context"
	"errors"

	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInstanceNotFound = errors.New("service type instance not found")
)

// ServiceTypeInstanceFilter contains optional fields for filtering instance queries.
// nil fields are ignored (not filtered).
type ServiceTypeInstanceFilter struct {
	ProviderName *string
}

// Pagination contains options for paginated queries.
type Pagination struct {
	Limit  int
	Offset int
}

type ServiceTypeInstance interface {
	List(ctx context.Context, filter *ServiceTypeInstanceFilter, pagination *Pagination) (model.ServiceTypeInstanceList, error)
	Create(ctx context.Context, instance model.ServiceTypeInstance) (*model.ServiceTypeInstance, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Get(ctx context.Context, id uuid.UUID) (*model.ServiceTypeInstance, error)
	ExistsByID(ctx context.Context, id uuid.UUID) (bool, error)
}

type ServiceTypeInstanceStore struct {
	db *gorm.DB
}

var _ ServiceTypeInstance = (*ServiceTypeInstanceStore)(nil)

func NewServiceTypeInstance(db *gorm.DB) ServiceTypeInstance {
	return &ServiceTypeInstanceStore{db: db}
}

func (s *ServiceTypeInstanceStore) List(
	ctx context.Context, filter *ServiceTypeInstanceFilter,
	pagination *Pagination) (model.ServiceTypeInstanceList, error) {

	var instances model.ServiceTypeInstanceList
	query := s.db.WithContext(ctx)

	if filter != nil {
		if filter.ProviderName != nil {
			query = query.Where(&model.ServiceTypeInstance{ProviderName: *filter.ProviderName})
		}
	}

	// Apply consistent ordering for pagination
	query = query.Order("create_time ASC, id ASC")

	if pagination != nil {
		query = query.Limit(pagination.Limit).Offset(pagination.Offset)
	}

	if err := query.Find(&instances).Error; err != nil {
		return nil, err
	}
	return instances, nil
}

func (s *ServiceTypeInstanceStore) Create(ctx context.Context, instance model.ServiceTypeInstance) (*model.ServiceTypeInstance, error) {
	if err := s.db.WithContext(ctx).Clauses(clause.Returning{}).Create(&instance).Error; err != nil {
		return nil, err
	}
	return &instance, nil
}

func (s *ServiceTypeInstanceStore) Delete(ctx context.Context, id uuid.UUID) error {
	result := s.db.WithContext(ctx).Delete(&model.ServiceTypeInstance{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrInstanceNotFound
	}
	return nil
}

func (s *ServiceTypeInstanceStore) Get(ctx context.Context, id uuid.UUID) (*model.ServiceTypeInstance, error) {
	var instance model.ServiceTypeInstance
	if err := s.db.WithContext(ctx).First(&instance, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}
	return &instance, nil
}

func (s *ServiceTypeInstanceStore) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	var instance model.ServiceTypeInstance
	err := s.db.WithContext(ctx).Select("id").Where(&model.ServiceTypeInstance{ID: id}).Take(&instance).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrInstanceNotFound
		}
		return false, err
	}
	return true, nil
}
