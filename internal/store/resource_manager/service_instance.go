package store

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"

	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInstanceNotFound = errors.New("service type instance not found")
)

// ServiceTypeInstanceListOptions contains optional fields for listing instances.
type ServiceTypeInstanceListOptions struct {
	ProviderName *string
	PageSize     int
	PageToken    *string
}

// ServiceTypeInstanceListResult contains the result of a List operation.
type ServiceTypeInstanceListResult struct {
	Instances     model.ServiceTypeInstanceList
	NextPageToken *string
}

type ServiceTypeInstance interface {
	List(ctx context.Context, opts *ServiceTypeInstanceListOptions) (*ServiceTypeInstanceListResult, error)
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

func (s *ServiceTypeInstanceStore) List(ctx context.Context, opts *ServiceTypeInstanceListOptions) (*ServiceTypeInstanceListResult, error) {
	var instances model.ServiceTypeInstanceList
	query := s.db.WithContext(ctx)

	// Default page size
	pageSize := 50
	if opts != nil && opts.PageSize > 0 {
		pageSize = opts.PageSize
	}

	// Decode page token to get offset
	offset := 0
	if opts != nil && opts.PageToken != nil && *opts.PageToken != "" {
		decoded, err := base64.StdEncoding.DecodeString(*opts.PageToken)
		if err == nil {
			if parsedOffset, err := strconv.Atoi(string(decoded)); err == nil {
				offset = parsedOffset
			}
		}
	}

	// Apply filters
	if opts != nil && opts.ProviderName != nil && *opts.ProviderName != "" {
		query = query.Where("provider_name = ?", *opts.ProviderName)
	}

	// Apply consistent ordering for pagination
	query = query.Order("create_time ASC, id ASC")

	// Query with limit+1 to detect if there are more results
	query = query.Limit(pageSize + 1).Offset(offset)

	if err := query.Find(&instances).Error; err != nil {
		return nil, err
	}

	// Generate next page token if there are more results
	result := &ServiceTypeInstanceListResult{
		Instances: instances,
	}

	if len(instances) > pageSize {
		// Trim to requested page size
		result.Instances = instances[:pageSize]
		// Encode next offset as page token
		nextOffset := offset + pageSize
		encodedNextPageToken := base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(nextOffset)))
		result.NextPageToken = &encodedNextPageToken
	}

	return result, nil
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
			return false, nil
		}
		return false, err
	}
	return true, nil
}
