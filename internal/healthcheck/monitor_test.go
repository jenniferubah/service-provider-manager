package healthcheck_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/dcm-project/service-provider-manager/internal/config"
	"github.com/dcm-project/service-provider-manager/internal/healthcheck"
	"github.com/dcm-project/service-provider-manager/internal/store"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// testHealthCheckConfig returns a default config for testing
func testHealthCheckConfig() *config.HealthCheckConfig {
	return &config.HealthCheckConfig{
		Interval:               10 * time.Second,
		Timeout:                5 * time.Second,
		MaxConsecutiveFailures: 3,
		BaseBackoffInterval:    10 * time.Second,
		MaxBackoffInterval:     5 * time.Minute,
	}
}

// mockProviderStore implements store.Provider interface for testing
type mockProviderStore struct {
	providers           model.ProviderList
	healthStatusUpdates []healthStatusUpdate
}

type healthStatusUpdate struct {
	ID                  uuid.UUID
	Status              model.HealthStatus
	ConsecutiveFailures int
	NextCheck           time.Time
}

func (m *mockProviderStore) ListProvidersForHealthCheck(ctx context.Context, now time.Time) (model.ProviderList, error) {
	var result model.ProviderList
	for _, p := range m.providers {
		if p.NextHealthCheck == nil || !p.NextHealthCheck.After(now) {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProviderStore) UpdateHealthStatus(ctx context.Context, id uuid.UUID, status model.HealthStatus, consecutiveFailures int, nextCheck time.Time) error {
	m.healthStatusUpdates = append(m.healthStatusUpdates, healthStatusUpdate{
		ID:                  id,
		Status:              status,
		ConsecutiveFailures: consecutiveFailures,
		NextCheck:           nextCheck,
	})
	return nil
}

func (m *mockProviderStore) List(ctx context.Context, filter *store.ProviderFilter, pagination *store.Pagination) (model.ProviderList, error) {
	return m.providers, nil
}

func (m *mockProviderStore) Count(ctx context.Context, filter *store.ProviderFilter) (int64, error) {
	return int64(len(m.providers)), nil
}

func (m *mockProviderStore) ExistsByID(ctx context.Context, id uuid.UUID) (bool, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockProviderStore) Create(ctx context.Context, provider model.Provider) (*model.Provider, error) {
	return &provider, nil
}

func (m *mockProviderStore) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockProviderStore) Update(ctx context.Context, provider model.Provider) (*model.Provider, error) {
	return &provider, nil
}

func (m *mockProviderStore) Get(ctx context.Context, id uuid.UUID) (*model.Provider, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, nil
}

func (m *mockProviderStore) GetByName(ctx context.Context, name string) (*model.Provider, error) {
	for _, p := range m.providers {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, nil
}

var _ = Describe("Monitor", func() {
	var (
		cfg     *config.HealthCheckConfig
		monitor *healthcheck.Monitor
		ctx     context.Context
	)

	BeforeEach(func() {
		cfg = testHealthCheckConfig()
		ctx = context.Background()
	})

	Describe("CalculateNextCheckTime", func() {
		Context("for a Ready provider", func() {
			It("schedules next check at the configured interval", func() {
				mockStore := &mockProviderStore{}
				monitor = healthcheck.NewMonitor(mockStore, cfg)
				now := time.Now()

				nextCheck := monitor.CalculateNextCheckTime(now, model.HealthStatusReady, 0)

				Expect(nextCheck.Sub(now)).To(Equal(cfg.Interval))
			})
		})

		Context("for a NotReady provider with exponential backoff", func() {
			var (
				mockStore *mockProviderStore
				now       time.Time
			)

			BeforeEach(func() {
				mockStore = &mockProviderStore{}
				monitor = healthcheck.NewMonitor(mockStore, cfg)
				now = time.Now()
			})

			It("uses base backoff interval when just became NotReady (3 failures)", func() {
				nextCheck := monitor.CalculateNextCheckTime(now, model.HealthStatusNotReady, 3)
				Expect(nextCheck.Sub(now)).To(Equal(cfg.BaseBackoffInterval))
			})

			It("doubles backoff for 4 consecutive failures", func() {
				nextCheck := monitor.CalculateNextCheckTime(now, model.HealthStatusNotReady, 4)
				Expect(nextCheck.Sub(now)).To(Equal(cfg.BaseBackoffInterval * 2))
			})

			It("quadruples backoff for 5 consecutive failures", func() {
				nextCheck := monitor.CalculateNextCheckTime(now, model.HealthStatusNotReady, 5)
				Expect(nextCheck.Sub(now)).To(Equal(cfg.BaseBackoffInterval * 4))
			})

			It("caps backoff at max interval for many failures", func() {
				nextCheck := monitor.CalculateNextCheckTime(now, model.HealthStatusNotReady, 100)
				Expect(nextCheck.Sub(now)).To(Equal(cfg.MaxBackoffInterval))
			})
		})
	})

	Describe("CheckProviders", func() {
		Context("with a healthy provider", func() {
			It("sets status to Ready with zero consecutive failures", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/health" {
						w.WriteHeader(http.StatusOK)
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
				defer server.Close()

				providerID := uuid.New()
				mockStore := &mockProviderStore{
					providers: model.ProviderList{
						{
							ID:           providerID,
							Name:         "test-provider",
							Endpoint:     server.URL,
							HealthStatus: model.HealthStatusReady,
						},
					},
				}

				monitor = healthcheck.NewMonitor(mockStore, cfg)
				monitor.CheckProviders(ctx)

				Expect(mockStore.healthStatusUpdates).To(HaveLen(1))
				update := mockStore.healthStatusUpdates[0]
				Expect(update.Status).To(Equal(model.HealthStatusReady))
				Expect(update.ConsecutiveFailures).To(Equal(0))
			})
		})

		Context("with an unhealthy provider", func() {
			It("becomes NotReady after reaching max consecutive failures", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				defer server.Close()

				providerID := uuid.New()
				mockStore := &mockProviderStore{
					providers: model.ProviderList{
						{
							ID:                  providerID,
							Name:                "test-provider",
							Endpoint:            server.URL,
							HealthStatus:        model.HealthStatusReady,
							ConsecutiveFailures: 2, // Already 2 failures, this will be the 3rd
						},
					},
				}

				monitor = healthcheck.NewMonitor(mockStore, cfg)
				monitor.CheckProviders(ctx)

				Expect(mockStore.healthStatusUpdates).To(HaveLen(1))
				update := mockStore.healthStatusUpdates[0]
				Expect(update.Status).To(Equal(model.HealthStatusNotReady))
				Expect(update.ConsecutiveFailures).To(Equal(3))
			})

			It("stays Ready until reaching max consecutive failures", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
				defer server.Close()

				providerID := uuid.New()
				mockStore := &mockProviderStore{
					providers: model.ProviderList{
						{
							ID:                  providerID,
							Name:                "test-provider",
							Endpoint:            server.URL,
							HealthStatus:        model.HealthStatusReady,
							ConsecutiveFailures: 1, // Only 1 failure so far
						},
					},
				}

				monitor = healthcheck.NewMonitor(mockStore, cfg)
				monitor.CheckProviders(ctx)

				Expect(mockStore.healthStatusUpdates).To(HaveLen(1))
				update := mockStore.healthStatusUpdates[0]
				Expect(update.Status).To(Equal(model.HealthStatusReady))
				Expect(update.ConsecutiveFailures).To(Equal(2))
			})
		})

		Context("with a recovered provider", func() {
			It("resets to Ready with zero consecutive failures", func() {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()

				providerID := uuid.New()
				mockStore := &mockProviderStore{
					providers: model.ProviderList{
						{
							ID:                  providerID,
							Name:                "test-provider",
							Endpoint:            server.URL,
							HealthStatus:        model.HealthStatusNotReady,
							ConsecutiveFailures: 5, // Was failing, now healthy
						},
					},
				}

				monitor = healthcheck.NewMonitor(mockStore, cfg)
				monitor.CheckProviders(ctx)

				Expect(mockStore.healthStatusUpdates).To(HaveLen(1))
				update := mockStore.healthStatusUpdates[0]
				Expect(update.Status).To(Equal(model.HealthStatusReady))
				Expect(update.ConsecutiveFailures).To(Equal(0))
			})
		})
	})
})
