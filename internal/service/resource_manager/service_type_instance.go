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
			return nil, service.NewNotFoundError(fmt.Sprintf("provider '%s' not found", providerName))
		}
		return nil, service.NewInternalError(fmt.Sprintf("failed to retrieve provider: %v", err))
	}

	// Check Provider if provider is not in ready state
	if provider.HealthStatus != model.HealthStatusReady {
		return nil, service.NewProviderError(fmt.Sprintf("provider '%s' is not in ready state (current status: %s)", providerName, provider.HealthStatus))
	}

	// Resolve instance ID
	instanceID, err := s.resolveInstanceID(ctx, queryID)
	if err != nil {
		return nil, err
	}
	instanceIDStr := instanceID.String()

	// Convert spec to JSON
	specJSON, err := json.Marshal(request.Spec)
	if err != nil {
		return nil, service.NewValidationError(fmt.Sprintf("invalid spec: %v", err))
	}

	// Send request to provider endpoint with the resolved ID
	providerResponse, err := s.createInstanceWithProvider(ctx, provider.Endpoint, request, &instanceIDStr)
	if err != nil {
		return nil, service.NewProviderError(fmt.Sprintf("Error from Provider (%s): %v", providerName, err))
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
		return nil, service.NewInternalError(fmt.Sprintf("failed to create database record for instance %s: %v", providerResponse.ID, err))
	}

	log.Printf("Inserted instance into DB: %s", created.ID)

	// Return the created instance
	return ModelToAPI(created), nil
}

// GetInstance retrieves an instance by ID
func (s *InstanceService) GetInstance(ctx context.Context, instanceID string) (*resource_manager.ServiceTypeInstance, error) {
	id, err := uuid.Parse(instanceID)
	if err != nil {
		return nil, service.NewValidationError("invalid instance ID format")
	}

	instance, err := s.store.ServiceTypeInstance().Get(ctx, id)
	if err != nil {
		if errors.Is(err, rmstore.ErrInstanceNotFound) {
			return nil, service.NewNotFoundError(fmt.Sprintf("instance %s not found", instanceID))
		}
		return nil, service.NewInternalError(fmt.Sprintf("failed to retrieve instance: %v", err))
	}

	return ModelToAPI(instance), nil
}

// ListInstances returns instances with optional filtering and pagination
func (s *InstanceService) ListInstances(ctx context.Context, providerName *string, maxPageSize *int, pageToken *string) (*resource_manager.ServiceTypeInstanceList, error) {
	opts := &rmstore.ServiceTypeInstanceListOptions{
		ProviderName: providerName,
	}

	// Apply max page size (default 50, max 100)
	if maxPageSize != nil {
		if *maxPageSize > 0 && *maxPageSize <= 100 {
			opts.PageSize = *maxPageSize
		} else {
			return nil, service.NewValidationError("page size must be between 1 and 100")
		}
	}

	// Apply page token
	if pageToken != nil && *pageToken != "" {
		opts.PageToken = pageToken
	}

	result, err := s.store.ServiceTypeInstance().List(ctx, opts)
	if err != nil {
		return nil, service.NewInternalError(fmt.Sprintf("failed to list instances: %v", err))
	}

	// Convert to API types
	apiInstances := make([]resource_manager.ServiceTypeInstance, len(result.Instances))
	for i, inst := range result.Instances {
		apiInstances[i] = *ModelToAPI(&inst)
	}

	apiResult := &resource_manager.ServiceTypeInstanceList{
		Instances:     &apiInstances,
		NextPageToken: result.NextPageToken,
	}

	return apiResult, nil
}

// DeleteInstance removes an instance by ID
func (s *InstanceService) DeleteInstance(ctx context.Context, instanceID string) error {
	id, err := uuid.Parse(instanceID)
	if err != nil {
		return service.NewValidationError("invalid instance ID format")
	}

	// Get instance to find provider
	instance, err := s.store.ServiceTypeInstance().Get(ctx, id)
	if err != nil {
		if errors.Is(err, rmstore.ErrInstanceNotFound) {
			return service.NewNotFoundError(fmt.Sprintf("instance %s not found", instanceID))
		}
		return service.NewInternalError(fmt.Sprintf("failed to retrieve instance: %v", err))
	}

	// Get provider to send delete request
	provider, err := s.store.Provider().GetByName(ctx, instance.ProviderName)
	if err != nil && !errors.Is(err, store.ErrProviderNotFound) {
		return service.NewInternalError(fmt.Sprintf("failed to retrieve provider: %v", err))
	}

	// Send delete request to provider if provider still exists
	if provider != nil {
		err = s.deleteInstanceWithProvider(ctx, provider.Endpoint, instanceID)
		if err != nil {
			log.Printf("Error: failed to delete instance (%s) from provider (%s): %v", instanceID, provider.Name, err)
			return service.NewProviderError(fmt.Sprintf("failed to delete instance (%s): %v", instanceID, err))
		}
		log.Printf("Deleted instance (%s) from SP (%s)", instanceID, provider.Name)
	}

	// Delete from database
	err = s.store.ServiceTypeInstance().Delete(ctx, id)
	if err != nil {
		// add re-try mechanism
		return service.NewInternalError(fmt.Sprintf("failed to delete database record for instance %s: %v", instanceID, err))
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
		return uuid.UUID{}, service.NewValidationError("invalid instance ID format")
	}

	exists, err := s.store.ServiceTypeInstance().ExistsByID(ctx, requestedID)
	if err != nil {
		return uuid.UUID{}, service.NewInternalError(fmt.Sprintf("failed to check instance existence: %v", err))
	}
	if exists {
		return uuid.UUID{}, service.NewConflictError(fmt.Sprintf("instance with ID '%s' already exists", requestedID))
	}

	return requestedID, nil
}

// createInstanceWithProvider sends the create request to the provider's endpoint
func (s *InstanceService) createInstanceWithProvider(ctx context.Context, endpoint string, request *resource_manager.ServiceTypeInstance, id *string) (*ProviderResponse, error) {

	var providerResp ProviderResponse

	resp, err := s.httpClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParam("id", *id).
		SetBody(request.Spec).
		SetResult(&providerResp).
		Post(endpoint)

	if err != nil {
		return nil, service.NewProviderError(fmt.Sprintf("failed to connect to provider: %v", err))
	}

	if resp.IsError() {
		return nil, service.NewProviderError(fmt.Sprintf("provider returned error: %s", resp.Status()))
	}

	return &providerResp, nil
}

// deleteInstanceWithProvider sends the delete request to the provider's endpoint
func (s *InstanceService) deleteInstanceWithProvider(ctx context.Context, endpoint string, instanceID string) error {
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
