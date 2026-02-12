package resource_manager

import (
	"context"

	server "github.com/dcm-project/service-provider-manager/internal/api/server/resource_manager"
	rmsvc "github.com/dcm-project/service-provider-manager/internal/service/resource_manager"
)

// Handler implements the generated StrictServerInterface for the Resource Manager API.
type Handler struct {
	instanceService *rmsvc.InstanceService
}

// NewHandler creates a new Handler with the given instance service.
func NewHandler(instanceService *rmsvc.InstanceService) *Handler {
	return &Handler{instanceService: instanceService}
}

// Ensure Handler implements StrictServerInterface
var _ server.StrictServerInterface = (*Handler)(nil)

// GetHealth returns the health status of the service.
func (h *Handler) GetHealth(ctx context.Context, request server.GetHealthRequestObject) (server.GetHealthResponseObject, error) {
	status := "ok"
	path := "health"
	return server.GetHealth200JSONResponse{Status: &status, Path: &path}, nil
}

// ListInstances returns a paginated list of service type instances.
func (h *Handler) ListInstances(ctx context.Context, request server.ListInstancesRequestObject) (server.ListInstancesResponseObject, error) {

	result, err := h.instanceService.ListInstances(
		ctx,
		request.Params.Provider,
		request.Params.MaxPageSize,
		request.Params.PageToken,
	)
	if err != nil {
		return handleListInstancesError(err), nil
	}

	instances := convertAPIListToServer(result.Instances)
	response := server.ListInstances200JSONResponse{
		Instances:     &instances,
		NextPageToken: result.NextPageToken}

	return response, nil
}

// CreateInstance creates a new service type instance.
func (h *Handler) CreateInstance(ctx context.Context, request server.CreateInstanceRequestObject) (server.CreateInstanceResponseObject, error) {
	instance := convertServerToAPI(request.Body)

	result, err := h.instanceService.CreateInstance(ctx, instance, request.Params.Id)
	if err != nil {
		return handleCreateInstanceError(err), nil
	}

	return server.CreateInstance201JSONResponse(convertAPIToServer(result)), nil
}

// GetInstance retrieves a service type instance by ID.
func (h *Handler) GetInstance(ctx context.Context, request server.GetInstanceRequestObject) (server.GetInstanceResponseObject, error) {
	result, err := h.instanceService.GetInstance(ctx, request.InstanceId)
	if err != nil {
		return handleGetInstanceError(err), nil
	}

	return server.GetInstance200JSONResponse(convertAPIToServer(result)), nil
}

// DeleteInstance deletes a service type instance by ID.
func (h *Handler) DeleteInstance(ctx context.Context, request server.DeleteInstanceRequestObject) (server.DeleteInstanceResponseObject, error) {
	err := h.instanceService.DeleteInstance(ctx, request.InstanceId)
	if err != nil {
		return handleDeleteInstanceError(err), nil
	}

	return server.DeleteInstance204Response{}, nil
}
