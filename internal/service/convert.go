package service

import (
	"time"

	"github.com/dcm-project/service-provider-manager/internal/api/server"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ModelToProvider converts a database model to an API response type
func ModelToProvider(m *model.Provider) *server.Provider {
	id := openapi_types.UUID(m.ID)
	return &server.Provider{
		Id:            &id,
		Name:          m.Name,
		ServiceType:   m.ServiceType,
		SchemaVersion: m.SchemaVersion,
		Endpoint:      m.Endpoint,
		CreateTime:    ptrTime(m.CreateTime),
		UpdateTime:    ptrTime(m.UpdateTime),
	}
}

// ModelToProviderWithStatus converts a database model to an API response with status
func ModelToProviderWithStatus(m *model.Provider, status server.ProviderStatus) *server.Provider {
	p := ModelToProvider(m)
	p.Status = &status
	return p
}

// ProviderToModel converts an API request to a database model
func ProviderToModel(req *server.Provider, id uuid.UUID) model.Provider {
	now := time.Now()
	return model.Provider{
		ID:            id,
		Name:          req.Name,
		ServiceType:   req.ServiceType,
		SchemaVersion: req.SchemaVersion,
		Endpoint:      req.Endpoint,
		CreateTime:    now,
		UpdateTime:    now,
	}
}

// Helper functions for pointer conversions

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
