package resource_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"
	"github.com/dcm-project/service-provider-manager/internal/service"
	"github.com/dcm-project/service-provider-manager/internal/store"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
	rmstore "github.com/dcm-project/service-provider-manager/internal/store/resource_manager"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type InstanceService struct {
	store      store.Store
	httpClient *resty.Client
}

func NewInstanceService(store store.Store) *InstanceService {
	client := resty.New().
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second)

	return &InstanceService{
		store:      store,
		httpClient: client,
	}
}

// CreateInstance creates a new service type instance
func (s *InstanceService) CreateInstance(ctx context.Context, request *resource_manager.ServiceTypeInstance, queryID *string) (*resource_manager.ServiceTypeInstance, error) {
	// Get provider information
	providerName := request.ProviderName
	provider, err := s.store.Provider().GetByName(ctx, providerName)
	if err != nil {
		if errors.Is(err, store.ErrProviderNotFound) {
			return nil, &service.ServiceError{
				Code:    service.ErrCodeNotFound,
				Message: fmt.Sprintf("provider '%s' not found", providerName),
			}
		}
		return nil, err
	}

	// Check Provider if provider is not in ready state
	if provider.HealthStatus != model.HealthStatusReady {
		return nil, &service.ServiceError{
			Code:    service.ErrCodeProviderError,
			Message: fmt.Sprintf("provider '%s' is not in ready state (current status: %s)", providerName, provider.HealthStatus),
		}
	}

	// Resolve instance ID
	instanceID, err := s.resolveInstanceID(ctx, queryID)
	if err != nil {
		return nil, err
	}

	// Convert spec to JSON
	specJSON, err := json.Marshal(request.Spec)
	if err != nil {
		return nil, &service.ServiceError{
			Code:    service.ErrCodeValidation,
			Message: fmt.Sprintf("invalid spec: %v", err),
		}
	}

	// Send request to provider endpoint
	providerResponse, err := s.sendToProvider(ctx, provider.Endpoint, request)
	if err != nil {
		return nil, &service.ServiceError{
			Code:    service.ErrCodeProviderError,
			Message: fmt.Sprintf("Error from Provider (%s): %v", providerName, err),
		}
	}
	log.Printf("Created instance: %s for provider: %s", providerResponse.ID, providerName)

	// Create instance in database
	instance := model.ServiceTypeInstance{
		ID:           instanceID,
		ProviderName: providerName,
		Status:       providerResponse.Status,
		Spec:         datatypes.JSON(specJSON),
	}

	created, err := s.store.ServiceTypeInstance().Create(ctx, instance)
	if err != nil {
		// add re-try mechanism
		return nil, &service.ServiceError{
			Code:    service.ErrCodeInternal,
			Message: fmt.Sprintf("failed to create database record for instance %s: %v", providerResponse.ID, err),
		}
	}

	log.Printf("Inserted instance into DB: %s", created.ID)

	// Return the created instance
	return ModelToAPI(created), nil
}

// GetInstance retrieves an instance by ID
func (s *InstanceService) GetInstance(ctx context.Context, instanceID string) (*resource_manager.ServiceTypeInstance, error) {
	id, err := uuid.Parse(instanceID)
	if err != nil {
		return nil, &service.ServiceError{
			Code:    service.ErrCodeValidation,
			Message: "invalid instance ID format",
		}
	}

	instance, err := s.store.ServiceTypeInstance().Get(ctx, id)
	if err != nil {
		if errors.Is(err, rmstore.ErrInstanceNotFound) {
			return nil, &service.ServiceError{
				Code:    service.ErrCodeNotFound,
				Message: fmt.Sprintf("instance %s not found", instanceID),
			}
		}
		return nil, err
	}

	return ModelToAPI(instance), nil
}

// ListInstances returns instances with optional filtering and pagination
func (s *InstanceService) ListInstances(ctx context.Context, providerName *string, maxPageSize *int, pageToken string) (*resource_manager.ServiceTypeInstanceList, error) {
	var filter *rmstore.ServiceTypeInstanceFilter
	if providerName != nil && *providerName != "" {
		filter = &rmstore.ServiceTypeInstanceFilter{ProviderName: providerName}
	}

	// Apply pagination
	limit := 100
	if maxPageSize != nil && *maxPageSize > 0 && *maxPageSize < 100 {
		limit = *maxPageSize
	}

	offset := 0
	if pageToken != "" {
		decoded, err := service.DecodePageToken(pageToken)
		if err != nil {
			return nil, &service.ServiceError{
				Code: service.ErrCodeValidation, Message: "invalid page_token"}
		}
		offset = decoded
	}

	pagination := &rmstore.Pagination{Limit: limit, Offset: offset}

	instances, err := s.store.ServiceTypeInstance().List(ctx, filter, pagination)
	if err != nil {
		return nil, err
	}

	// Convert to API types
	result := make([]resource_manager.ServiceTypeInstance, len(instances))
	for i, inst := range instances {
		result[i] = *ModelToAPI(&inst)
	}

	return &resource_manager.ServiceTypeInstanceList{
		Instances: &result,
	}, nil
}

// DeleteInstance removes an instance by ID
func (s *InstanceService) DeleteInstance(ctx context.Context, instanceID string) error {
	id, err := uuid.Parse(instanceID)
	if err != nil {
		return &service.ServiceError{
			Code:    service.ErrCodeValidation,
			Message: "invalid instance ID format",
		}
	}

	// Get instance to find provider
	instance, err := s.store.ServiceTypeInstance().Get(ctx, id)
	if err != nil {
		if errors.Is(err, rmstore.ErrInstanceNotFound) {
			return &service.ServiceError{
				Code:    service.ErrCodeNotFound,
				Message: fmt.Sprintf("instance %s not found", instanceID),
			}
		}
		return err
	}

	// Get provider to send delete request
	provider, err := s.store.Provider().GetByName(ctx, instance.ProviderName)
	if err != nil && !errors.Is(err, store.ErrProviderNotFound) {
		return err
	}

	// Send delete request to provider if provider still exists
	if provider != nil {
		err = s.sendDeleteToProvider(ctx, provider.Endpoint, instanceID)
		if err != nil {
			log.Printf("Error: failed to delete instance (%s) from provider (%s): %v", instanceID, provider.Name, err)
			if errors.Is(err, rmstore.ErrInstanceNotFound) {
				return &service.ServiceError{
					Code:    service.ErrCodeProviderError,
					Message: fmt.Sprintf("failed to delete instance (%s): %v", instanceID, err),
				}
			}
		}
		log.Printf("Deleted instance (%s) from SP (%s)", instanceID, provider.Name)
	}

	// Delete from database
	err = s.store.ServiceTypeInstance().Delete(ctx, id)
	if err != nil {
		// add re-try mechanism
		return &service.ServiceError{
			Code:    service.ErrCodeInternal,
			Message: fmt.Sprintf("failed to delete database record for instance %s: %v", instanceID, err),
		}
	}

	log.Printf("Deleted instance from DB record: %s", instanceID)
	return nil
}

// resolveInstanceID returns the requested ID after checking for conflicts, or generates a new one
func (s *InstanceService) resolveInstanceID(ctx context.Context, queryID *string) (uuid.UUID, error) {

	if queryID == nil || *queryID == "" {
		return uuid.New(), nil
	}

	requestedID, err := uuid.Parse(*queryID)
	if err != nil {
		return uuid.UUID{}, &service.ServiceError{
			Code:    service.ErrCodeValidation,
			Message: "invalid instance ID format",
		}
	}

	exists, err := s.store.ServiceTypeInstance().ExistsByID(ctx, requestedID)
	if err != nil {
		return uuid.UUID{}, err
	}
	if exists {
		return uuid.UUID{}, &service.ServiceError{
			Code:    service.ErrCodeConflict,
			Message: fmt.Sprintf("instance with ID '%s' already exists", requestedID),
		}
	}

	return requestedID, nil
}

// sendToProvider sends the create request to the provider's endpoint
func (s *InstanceService) sendToProvider(ctx context.Context, endpoint string, request *resource_manager.ServiceTypeInstance) (*ProviderResponse, error) {

	var providerResp ProviderResponse

	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(request).
		SetResult(&providerResp).
		Post(endpoint)

	if err != nil {
		return nil, &service.ServiceError{
			Code:    service.ErrCodeProviderError,
			Message: fmt.Sprintf("failed to connect to provider: %v", err),
		}
	}

	if resp.IsError() {
		return nil, &service.ServiceError{
			Code:    service.ErrCodeProviderError,
			Message: fmt.Sprintf("provider returned error: %s", resp.Status()),
		}
	}

	return &providerResp, nil
}

// sendDeleteToProvider sends the delete request to the provider's endpoint
func (s *InstanceService) sendDeleteToProvider(ctx context.Context, endpoint string, instanceID string) error {
	resp, err := s.httpClient.R().
		SetContext(ctx).
		Delete(fmt.Sprintf("%s/%s", endpoint, instanceID))

	if err != nil {
		return fmt.Errorf("failed to connect to provider: %w", err)
	}

	if resp.IsError() && resp.StatusCode() != 404 {
		return fmt.Errorf("provider returned error: %s", resp.Status())
	}

	return nil
}
