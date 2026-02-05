package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type ServiceTypeInstance struct {
	ID           uuid.UUID      `gorm:"primaryKey;type:uuid"`
	ProviderName string         `gorm:"column:provider_name;not null"`
	Status       string         `gorm:"column:status;not null"`
	InstanceName string         `gorm:"column:instance_name;not null"`
	Spec         datatypes.JSON `gorm:"column:spec;not null"`
	CreateTime   time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdateTime   time.Time      `gorm:"column:update_time;autoUpdateTime"`
}

type ServiceTypeInstanceList []ServiceTypeInstance
