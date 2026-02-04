package resource_manager

import (
	"github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"
	server "github.com/dcm-project/service-provider-manager/internal/api/server/resource_manager"
)

// convertServerToAPI converts a server ServiceTypeInstance to an API ServiceTypeInstance.
func convertServerToAPI(src *server.ServiceTypeInstance) *resource_manager.ServiceTypeInstance {
	return &resource_manager.ServiceTypeInstance{
		Id:           src.Id,
		ProviderName: src.ProviderName,
		ServiceType:  src.ServiceType,
		Spec:         src.Spec,
	}
}

// convertAPIToServer converts an API ServiceTypeInstance to a server ServiceTypeInstance.
func convertAPIToServer(src *resource_manager.ServiceTypeInstance) server.ServiceTypeInstance {
	return server.ServiceTypeInstance{
		Id:           src.Id,
		Path:         src.Path,
		ProviderName: src.ProviderName,
		ServiceType:  src.ServiceType,
		Spec:         src.Spec,
		CreateTime:   src.CreateTime,
		UpdateTime:   src.UpdateTime,
	}
}

// convertAPIListToServer converts a slice of API ServiceTypeInstance to server ServiceTypeInstance.
func convertAPIListToServer(src *[]resource_manager.ServiceTypeInstance) []server.ServiceTypeInstance {
	if src == nil {
		return nil
	}
	result := make([]server.ServiceTypeInstance, len(*src))
	for i, inst := range *src {
		result[i] = convertAPIToServer(&inst)
	}
	return result
}
