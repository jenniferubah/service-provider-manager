package model

import (
	"time"

	"github.com/google/uuid"
)

// HealthStatus represents the health status of a provider
type HealthStatus string

const (
	// HealthStatusReady indicates the provider is healthy and ready to serve requests
	HealthStatusReady HealthStatus = "ready"
	// HealthStatusNotReady indicates the provider is not healthy or unreachable
	HealthStatusNotReady HealthStatus = "not_ready"
)

func (h HealthStatus) StringPtr() *string {
	s := string(h)
	return &s
}

type Provider struct {
	ID            uuid.UUID `gorm:"primaryKey;type:uuid"`
	Name          string    `gorm:"uniqueIndex;not null"`
	ServiceType   string    `gorm:"column:service_type;not null"`
	SchemaVersion string    `gorm:"column:schema_version;not null"`
	Endpoint      string    `gorm:"column:endpoint;not null"`
	CreateTime    time.Time `gorm:"column:create_time;autoCreateTime"`
	UpdateTime    time.Time `gorm:"column:update_time;autoUpdateTime"`

	// Health check fields
	HealthStatus        HealthStatus `gorm:"column:health_status;default:ready"`
	ConsecutiveFailures int          `gorm:"column:consecutive_failures;default:0"`
	NextHealthCheck     *time.Time   `gorm:"column:next_health_check"`
}

type ProviderList []Provider
