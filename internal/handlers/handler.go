package handlers

import (
	"context"

	"github.com/dcm-project/service-provider-manager/internal/api/server"
	"github.com/dcm-project/service-provider-manager/internal/service"
)

// Handler implements the generated StrictServerInterface for the Provider API.
type Handler struct {
	providerService *service.ProviderService
}

// NewHandler creates a new Handler with the given provider service.
func NewHandler(providerService *service.ProviderService) *Handler {
	return &Handler{providerService: providerService}
}

// Ensure Handler implements StrictServerInterface
var _ server.StrictServerInterface = (*Handler)(nil)

func (h *Handler) GetHealth(ctx context.Context, request server.GetHealthRequestObject) (server.GetHealthResponseObject, error) {
	status := "ok"
	path := "health"
	return server.GetHealth200JSONResponse{Status: &status, Path: &path}, nil
}

func (h *Handler) ListProviders(ctx context.Context, request server.ListProvidersRequestObject) (server.ListProvidersResponseObject, error) {
	var serviceType string
	if request.Params.Type != nil {
		serviceType = *request.Params.Type
	}

	providers, err := h.providerService.ListProviders(ctx, serviceType)
	if err != nil {
		return server.ListProviders400ApplicationProblemPlusJSONResponse(newError("list-error", "Failed to list providers", err.Error(), 400)), nil
	}

	return server.ListProviders200JSONResponse{Providers: &providers}, nil
}

func (h *Handler) CreateProvider(ctx context.Context, request server.CreateProviderRequestObject) (server.CreateProviderResponseObject, error) {
	response, err := h.providerService.RegisterProvider(ctx, request.Body, request.Params.Id)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			switch svcErr.Code {
			case service.ErrCodeValidation:
				return server.CreateProvider400ApplicationProblemPlusJSONResponse(newError("validation-error", "Validation failed", svcErr.Message, 400)), nil
			case service.ErrCodeConflict:
				return server.CreateProvider409ApplicationProblemPlusJSONResponse(newError("conflict", "Resource conflict", svcErr.Message, 409)), nil
			}
		}
		return server.CreateProvider400ApplicationProblemPlusJSONResponse(newError("create-error", "Failed to create provider", err.Error(), 400)), nil
	}

	if response.Status != nil && *response.Status == server.Updated {
		return server.CreateProvider200JSONResponse(*response), nil
	}
	return server.CreateProvider201JSONResponse(*response), nil
}

func (h *Handler) GetProvider(ctx context.Context, request server.GetProviderRequestObject) (server.GetProviderResponseObject, error) {
	provider, err := h.providerService.GetProvider(ctx, request.ProviderId.String())
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok && svcErr.Code == service.ErrCodeNotFound {
			return server.GetProvider404ApplicationProblemPlusJSONResponse(newError("not-found", "Provider not found", svcErr.Message, 404)), nil
		}
		return server.GetProvider400ApplicationProblemPlusJSONResponse(newError("get-error", "Failed to get provider", err.Error(), 400)), nil
	}

	return server.GetProvider200JSONResponse(*provider), nil
}

func (h *Handler) ApplyProvider(ctx context.Context, request server.ApplyProviderRequestObject) (server.ApplyProviderResponseObject, error) {
	provider, err := h.providerService.UpdateProvider(ctx, request.ProviderId.String(), request.Body)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			switch svcErr.Code {
			case service.ErrCodeNotFound:
				return server.ApplyProvider404ApplicationProblemPlusJSONResponse(newError("not-found", "Provider not found", svcErr.Message, 404)), nil
			case service.ErrCodeConflict:
				return server.ApplyProvider409ApplicationProblemPlusJSONResponse(newError("conflict", "Name conflict", svcErr.Message, 409)), nil
			}
		}
		return server.ApplyProvider400ApplicationProblemPlusJSONResponse(newError("update-error", "Failed to update provider", err.Error(), 400)), nil
	}

	return server.ApplyProvider200JSONResponse(*provider), nil
}

func (h *Handler) DeleteProvider(ctx context.Context, request server.DeleteProviderRequestObject) (server.DeleteProviderResponseObject, error) {
	err := h.providerService.DeleteProvider(ctx, request.ProviderId.String())
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok && svcErr.Code == service.ErrCodeNotFound {
			return server.DeleteProvider404ApplicationProblemPlusJSONResponse(newError("not-found", "Provider not found", svcErr.Message, 404)), nil
		}
		return server.DeleteProvider400ApplicationProblemPlusJSONResponse(newError("delete-error", "Failed to delete provider", err.Error(), 400)), nil
	}

	return server.DeleteProvider204Response{}, nil
}

func newError(errType, title, detail string, status int) server.Error {
	return server.Error{
		Type:   errType,
		Title:  title,
		Detail: &detail,
		Status: &status,
	}
}
