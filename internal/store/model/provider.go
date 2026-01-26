package model

import (
	"time"

	"github.com/google/uuid"
)

type Provider struct {
	ID            uuid.UUID `gorm:"primaryKey;type:uuid"`
	Name          string    `gorm:"uniqueIndex;not null"`
	ServiceType   string    `gorm:"column:service_type;not null"`
	SchemaVersion string    `gorm:"column:schema_version;not null"`
	Endpoint      string    `gorm:"column:endpoint;not null"`
	CreateTime    time.Time `gorm:"column:create_time;autoCreateTime"`
	UpdateTime    time.Time `gorm:"column:update_time;autoUpdateTime"`
}

type ProviderList []Provider

