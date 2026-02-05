package resource_manager

import (
	"encoding/json"
	"fmt"

	"github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"
	"github.com/dcm-project/service-provider-manager/internal/service"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
)

// ProviderResponse represents the response from a provider during instance creation.
type ProviderResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ModelToAPI converts a database model to an API response type.
func ModelToAPI(instance *model.ServiceTypeInstance) *resource_manager.ServiceTypeInstance {
	id := instance.ID.String()
	path := fmt.Sprintf("service-type-instances/%s", id)

	var spec map[string]interface{}
	_ = json.Unmarshal(instance.Spec, &spec)

	return &resource_manager.ServiceTypeInstance{
		Id:           &id,
		Path:         &path,
		ProviderName: instance.ProviderName,
		Status:       &instance.Status,
		Spec:         spec,
		CreateTime:   service.PtrTime(instance.CreateTime),
		UpdateTime:   service.PtrTime(instance.UpdateTime),
	}
}
