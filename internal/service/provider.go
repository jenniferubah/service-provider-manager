package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/dcm-project/service-provider-manager/internal/api/server"
	"github.com/dcm-project/service-provider-manager/internal/store"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

const (
	defaultPageSize = 100
	maxPageSize     = 100
)

// ListResult contains the result of listing providers with pagination info.
type ListResult struct {
	Providers     []server.Provider
	NextPageToken string
}

// ProviderService handles business logic for provider management.
type ProviderService struct {
	store store.Store
}

// NewProviderService creates a new ProviderService with the given store.
func NewProviderService(store store.Store) *ProviderService {
	return &ProviderService{store: store}
}

// RegisterOrUpdateProvider implements idempotent provider registration per the DCM spec.
// Returns status "registered" for new providers, "updated" for existing ones.
// Returns ErrCodeConflict if name exists with different ID or ID exists with different name.
func (s *ProviderService) RegisterOrUpdateProvider(ctx context.Context, req *server.Provider, queryID *openapi_types.UUID) (*server.Provider, error) {
	requestedID := s.parseProviderID(req.Id, queryID)

	existing, err := s.findExistingByName(ctx, req.Name, requestedID)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		updated, err := s.updateExistingProvider(ctx, existing, req)
		if err != nil {
			return nil, err
		}
		return ModelToProviderWithStatus(updated, server.Updated), nil
	}

	providerID, err := s.resolveProviderID(ctx, requestedID)
	if err != nil {
		return nil, err
	}

	providerModel := ProviderToModel(req, providerID)
	created, err := s.store.Provider().Create(ctx, providerModel)
	if err != nil {
		return nil, err
	}

	log.Printf("Created provider: %s (%s)", created.Name, created.ID)
	return ModelToProviderWithStatus(created, server.Registered), nil
}

// parseProviderID extracts the provider ID from request body or query parameter.
func (s *ProviderService) parseProviderID(bodyID *openapi_types.UUID, queryID *openapi_types.UUID) *uuid.UUID {
	if bodyID != nil {
		id := uuid.UUID(*bodyID)
		return &id
	}
	if queryID != nil {
		id := uuid.UUID(*queryID)
		return &id
	}
	return nil
}

// findExistingByName returns the existing provider if name exists and is valid for update.
// Returns ErrCodeConflict if name exists with a different ID than requested.
func (s *ProviderService) findExistingByName(ctx context.Context, name string, requestedID *uuid.UUID) (*model.Provider, error) {
	existing, err := s.store.Provider().GetByName(ctx, name)
	if err != nil {
		if errors.Is(err, store.ErrProviderNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if requestedID != nil && existing.ID != *requestedID {
		return nil, &ServiceError{
			Code:    ErrCodeConflict,
			Message: fmt.Sprintf("name '%s' already exists with a different provider ID", name),
		}
	}

	return existing, nil
}

// resolveProviderID returns the requested ID after checking for conflicts, or generates a new one.
func (s *ProviderService) resolveProviderID(ctx context.Context, requestedID *uuid.UUID) (uuid.UUID, error) {
	if requestedID == nil {
		return uuid.New(), nil
	}

	exists, err := s.store.Provider().ExistsByID(ctx, *requestedID)
	if err != nil {
		return uuid.UUID{}, err
	}
	if exists {
		return uuid.UUID{}, &ServiceError{
			Code:    ErrCodeConflict,
			Message: fmt.Sprintf("provider with ID '%s' already exists", *requestedID),
		}
	}

	return *requestedID, nil
}

func (s *ProviderService) updateExistingProvider(ctx context.Context, existing *model.Provider, req *server.Provider) (*model.Provider, error) {
	existing.Name = req.Name
	existing.ServiceType = req.ServiceType
	existing.SchemaVersion = req.SchemaVersion
	existing.Endpoint = req.Endpoint
	existing.UpdateTime = time.Now()

	updated, err := s.store.Provider().Update(ctx, *existing)
	if err != nil {
		return nil, err
	}

	log.Printf("Updated provider: %s (%s)", updated.Name, updated.ID)
	return updated, nil
}

// GetProvider retrieves a provider by ID. Returns ErrCodeNotFound if not found.
func (s *ProviderService) GetProvider(ctx context.Context, providerID string) (*server.Provider, error) {
	id, err := uuid.Parse(providerID)
	if err != nil {
		return nil, &ServiceError{Code: ErrCodeValidation, Message: "invalid provider ID format"}
	}

	provider, err := s.store.Provider().Get(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrProviderNotFound) {
			return nil, &ServiceError{Code: ErrCodeNotFound, Message: fmt.Sprintf("provider %s not found", providerID)}
		}
		return nil, err
	}

	return ModelToProvider(provider), nil
}

// ListProviders returns providers with pagination support per AEP-158.
func (s *ProviderService) ListProviders(ctx context.Context, serviceType string, requestedPageSize int, pageToken string) (*ListResult, error) {
	// Validate and normalize page size per AEP-158
	pageSize := requestedPageSize
	if pageSize < 0 {
		return nil, &ServiceError{Code: ErrCodeValidation, Message: "max_page_size must not be negative"}
	}
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	// Decode page token to get offset
	offset := 0
	if pageToken != "" {
		decoded, err := decodePageToken(pageToken)
		if err != nil {
			return nil, &ServiceError{Code: ErrCodeValidation, Message: "invalid page_token"}
		}
		offset = decoded
	}

	// Build filter
	var filter *store.ProviderFilter
	if serviceType != "" {
		filter = &store.ProviderFilter{ServiceType: &serviceType}
	}

	// Get total count for next page calculation
	total, err := s.store.Provider().Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Fetch providers with pagination
	pagination := &store.Pagination{Limit: pageSize, Offset: offset}
	providers, err := s.store.Provider().List(ctx, filter, pagination)
	if err != nil {
		return nil, err
	}

	// Convert to API types
	result := make([]server.Provider, len(providers))
	for i, p := range providers {
		result[i] = *ModelToProvider(&p)
	}

	// Calculate next page token
	var nextPageToken string
	nextOffset := offset + len(providers)
	if int64(nextOffset) < total {
		nextPageToken = encodePageToken(nextOffset)
	}

	return &ListResult{
		Providers:     result,
		NextPageToken: nextPageToken,
	}, nil
}

func encodePageToken(offset int) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.Itoa(offset)))
}

func decodePageToken(token string) (int, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(decoded))
}

// UpdateProvider updates an existing provider. Returns ErrCodeNotFound if provider
// doesn't exist, or ErrCodeConflict if the new name is already taken.
func (s *ProviderService) UpdateProvider(ctx context.Context, providerID string, update *server.Provider) (*server.Provider, error) {
	id, err := uuid.Parse(providerID)
	if err != nil {
		return nil, &ServiceError{Code: ErrCodeValidation, Message: "invalid provider ID format"}
	}

	existing, err := s.store.Provider().Get(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrProviderNotFound) {
			return nil, &ServiceError{Code: ErrCodeNotFound, Message: fmt.Sprintf("provider %s not found", providerID)}
		}
		return nil, err
	}

	// Check for name conflict
	if update.Name != existing.Name {
		other, err := s.store.Provider().GetByName(ctx, update.Name)
		if err != nil && !errors.Is(err, store.ErrProviderNotFound) {
			return nil, err
		}
		if other != nil && other.ID != id {
			return nil, &ServiceError{Code: ErrCodeConflict, Message: fmt.Sprintf("name '%s' is already taken", update.Name)}
		}
	}

	updated, err := s.updateExistingProvider(ctx, existing, update)
	if err != nil {
		return nil, err
	}

	return ModelToProvider(updated), nil
}

// DeleteProvider removes a provider by ID. Returns ErrCodeNotFound if not found.
func (s *ProviderService) DeleteProvider(ctx context.Context, providerID string) error {
	id, err := uuid.Parse(providerID)
	if err != nil {
		return &ServiceError{Code: ErrCodeValidation, Message: "invalid provider ID format"}
	}

	err = s.store.Provider().Delete(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrProviderNotFound) {
			return &ServiceError{Code: ErrCodeNotFound, Message: fmt.Sprintf("provider %s not found", providerID)}
		}
		return err
	}

	log.Printf("Deleted provider: %s", providerID)
	return nil
}
