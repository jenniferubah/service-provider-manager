package config

import (
	"log"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Database    *DBConfig
	Service     *ServiceConfig
	HealthCheck *HealthCheckConfig
}

type HealthCheckConfig struct {
	Interval               time.Duration `envconfig:"HEALTH_CHECK_INTERVAL" default:"10s"`
	Timeout                time.Duration `envconfig:"HEALTH_CHECK_TIMEOUT" default:"5s"`
	MaxConsecutiveFailures int           `envconfig:"HEALTH_CHECK_MAX_CONSECUTIVE_FAILURES" default:"3"`
	BaseBackoffInterval    time.Duration `envconfig:"HEALTH_CHECK_BASE_BACKOFF_INTERVAL" default:"10s"`
	MaxBackoffInterval     time.Duration `envconfig:"HEALTH_CHECK_MAX_BACKOFF_INTERVAL" default:"5m"`
}

type DBConfig struct {
	Type     string `envconfig:"DB_TYPE" default:"pgsql"`
	Hostname string `envconfig:"DB_HOST" default:"localhost"`
	Port     string `envconfig:"DB_PORT" default:"5432"`
	Name     string `envconfig:"DB_NAME" default:"service-provider"`
	User     string `envconfig:"DB_USER"`
	Password string `envconfig:"DB_PASS"`
}

type ServiceConfig struct {
	Address  string `envconfig:"SVC_ADDRESS" default:":8080"`
	LogLevel string `envconfig:"SVC_LOG_LEVEL" default:"info"`
}

func Load() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}
	if cfg.Database.Type != "pgsql" && cfg.Database.Type != "sqlite" {
		log.Printf("WARNING: invalid DB_TYPE %q, defaulting to sqlite", cfg.Database.Type)
		cfg.Database.Type = "sqlite"
	}
	return cfg, nil
}
