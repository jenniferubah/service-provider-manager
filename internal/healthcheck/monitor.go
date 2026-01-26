package healthcheck

import (
	"context"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dcm-project/service-provider-manager/internal/config"
	"github.com/dcm-project/service-provider-manager/internal/store"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
)

// Monitor performs periodic health checks on registered service providers
type Monitor struct {
	store                  store.Provider
	httpClient             *http.Client
	interval               time.Duration
	stopCh                 chan struct{}
	wg                     sync.WaitGroup
	maxConsecutiveFailures int
	baseBackoffInterval    time.Duration
	maxBackoffInterval     time.Duration
}

// NewMonitor creates a new health check monitor
func NewMonitor(providerStore store.Provider, config *config.HealthCheckConfig) *Monitor {
	return &Monitor{
		store: providerStore,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		interval:               config.Interval,
		stopCh:                 make(chan struct{}),
		maxConsecutiveFailures: config.MaxConsecutiveFailures,
		baseBackoffInterval:    config.BaseBackoffInterval,
		maxBackoffInterval:     config.MaxBackoffInterval,
	}
}

// Start begins the health check monitoring loop
func (m *Monitor) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.run(ctx)
}

// Stop gracefully stops the health check monitor
func (m *Monitor) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

func (m *Monitor) run(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Run immediately on start
	m.CheckProviders(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.CheckProviders(ctx)
		}
	}
}

// CheckProviders checks all providers that are due for a health check
func (m *Monitor) CheckProviders(ctx context.Context) {
	now := time.Now()
	providers, err := m.store.ListProvidersForHealthCheck(ctx, now)
	if err != nil {
		log.Printf("Error listing providers for health check: %v", err)
		return
	}

	for _, provider := range providers {
		select {
		case <-ctx.Done():
			return
		default:
			m.checkProvider(ctx, provider)
		}
	}
}

func (m *Monitor) checkProvider(ctx context.Context, provider model.Provider) {
	now := time.Now()
	newStatus := model.HealthStatusReady
	consecutiveFailures := 0

	healthy := m.performHealthCheck(ctx, provider)
	if !healthy {
		consecutiveFailures = provider.ConsecutiveFailures + 1

		newStatus = provider.HealthStatus
		if consecutiveFailures >= m.maxConsecutiveFailures {
			newStatus = model.HealthStatusNotReady
		}
	}

	nextCheck := m.CalculateNextCheckTime(now, newStatus, consecutiveFailures)
	if err := m.store.UpdateHealthStatus(ctx, provider.ID, newStatus, consecutiveFailures, nextCheck); err != nil {
		log.Printf("Error updating health status for provider %s: %v", provider.Name, err)
		return
	}

	if provider.HealthStatus != newStatus {
		log.Printf("Provider %s health status changed: %s -> %s", provider.Name, provider.HealthStatus, newStatus)
	}
}

func (m *Monitor) performHealthCheck(ctx context.Context, provider model.Provider) bool {
	healthURL := strings.TrimRight(provider.Endpoint, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		log.Printf("Error creating health check request for provider %s: %v", provider.Name, err)
		return false
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		log.Printf("Health check failed for provider %s: %v", provider.Name, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}

	log.Printf("Health check failed for provider %s: status code %d", provider.Name, resp.StatusCode)
	return false
}

// CalculateNextCheckTime determines when the next health check should occur
// For Ready providers: standard interval (10 seconds)
// Exponential backoff for NotReady providers
// Formula: min(MaxBackoff, BaseInterval * 2^(failures - MaxConsecutiveFailures))
// This starts exponential backoff after the provider becomes NotReady
func (m *Monitor) CalculateNextCheckTime(now time.Time, status model.HealthStatus, consecutiveFailures int) time.Time {
	if status == model.HealthStatusReady {
		return now.Add(m.interval)
	}

	exponent := consecutiveFailures - m.maxConsecutiveFailures
	if exponent < 0 {
		exponent = 0
	}

	const maxExponent = 10
	if exponent > maxExponent {
		exponent = maxExponent
	}

	backoffMultiplier := math.Pow(2, float64(exponent))
	backoffDuration := time.Duration(float64(m.baseBackoffInterval) * backoffMultiplier)

	if backoffDuration > m.maxBackoffInterval {
		backoffDuration = m.maxBackoffInterval
	}

	return now.Add(backoffDuration)
}
