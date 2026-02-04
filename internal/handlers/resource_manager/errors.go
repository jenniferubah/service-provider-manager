package resource_manager

import (
	"errors"

	server "github.com/dcm-project/service-provider-manager/internal/api/server/resource_manager"
	"github.com/dcm-project/service-provider-manager/internal/service"
)

// newError creates an RFC 7807 compliant error response.
func newError(errType, title, detail string, status int) server.Error {
	return server.Error{
		Type:   errType,
		Title:  title,
		Detail: &detail,
		Status: &status,
	}
}

// handleListInstancesError converts a service error to a ListInstances response.
func handleListInstancesError(err error) server.ListInstancesResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) && svcErr.Code == service.ErrCodeValidation {
		return server.ListInstances400ApplicationProblemPlusJSONResponse(newError("validation-error", "Invalid request", svcErr.Message, 400))
	}
	return server.ListInstancesdefaultApplicationProblemPlusJSONResponse{
		Body:       newError("list-error", "Failed to list instances", err.Error(), 500),
		StatusCode: 500,
	}
}

// handleCreateInstanceError converts a service error to a CreateInstance response.
func handleCreateInstanceError(err error) server.CreateInstanceResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Code {
		case service.ErrCodeValidation:
			return server.CreateInstance400ApplicationProblemPlusJSONResponse(newError("validation-error", "Validation failed", svcErr.Message, 400))
		case service.ErrCodeNotFound:
			return server.CreateInstance400ApplicationProblemPlusJSONResponse(newError("not-found", "Resource not found", svcErr.Message, 400))
		case service.ErrCodeConflict:
			return server.CreateInstance409ApplicationProblemPlusJSONResponse(newError("conflict", "Resource conflict", svcErr.Message, 409))
		case service.ErrCodeProviderError:
			return server.CreateInstance422ApplicationProblemPlusJSONResponse(newError("provider-error", "Provider error", svcErr.Message, 422))
		}
	}
	return server.CreateInstancedefaultApplicationProblemPlusJSONResponse{
		Body:       newError("create-error", "Failed to create instance", err.Error(), 500),
		StatusCode: 500,
	}
}

// handleGetInstanceError converts a service error to a GetInstance response.
func handleGetInstanceError(err error) server.GetInstanceResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Code {
		case service.ErrCodeValidation:
			return server.GetInstance400ApplicationProblemPlusJSONResponse(newError("validation-error", "Invalid request", svcErr.Message, 400))
		case service.ErrCodeNotFound:
			return server.GetInstance404ApplicationProblemPlusJSONResponse(newError("not-found", "Instance not found", svcErr.Message, 404))
		}
	}
	return server.GetInstancedefaultApplicationProblemPlusJSONResponse{
		Body:       newError("get-error", "Failed to get instance", err.Error(), 500),
		StatusCode: 500,
	}
}

// handleDeleteInstanceError converts a service error to a DeleteInstance response.
func handleDeleteInstanceError(err error) server.DeleteInstanceResponseObject {
	var svcErr *service.ServiceError
	if errors.As(err, &svcErr) {
		switch svcErr.Code {
		case service.ErrCodeValidation:
			return server.DeleteInstance400ApplicationProblemPlusJSONResponse(newError("validation-error", "Invalid request", svcErr.Message, 400))
		case service.ErrCodeNotFound:
			return server.DeleteInstance404ApplicationProblemPlusJSONResponse(newError("not-found", "Instance not found", svcErr.Message, 404))
		}
	}
	return server.DeleteInstancedefaultApplicationProblemPlusJSONResponse{
		Body:       newError("delete-error", "Failed to delete instance", err.Error(), 500),
		StatusCode: 500,
	}
}
