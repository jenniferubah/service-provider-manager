package resource_manager_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/dcm-project/service-provider-manager/api/v1alpha1/resource_manager"
	"github.com/dcm-project/service-provider-manager/internal/service"
	rmsvc "github.com/dcm-project/service-provider-manager/internal/service/resource_manager"
	"github.com/dcm-project/service-provider-manager/internal/store"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ = Describe("InstanceService", func() {
	var (
		db              *gorm.DB
		dataStore       store.Store
		instanceService *rmsvc.InstanceService
		ctx             context.Context
		mockProvider    *httptest.Server
		providerCalled  bool
		deleteRequested bool
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.Provider{}, &model.ServiceTypeInstance{})).To(Succeed())

		// Create a mock provider server
		providerCalled = false
		deleteRequested = false
		mockProvider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				deleteRequested = true
				w.WriteHeader(http.StatusNoContent)
				return
			}
			providerCalled = true
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"id":     uuid.New().String(),
				"status": "PROVISIONING",
			})
		}))

		// Create a provider in the database
		provider := model.Provider{
			ID:           uuid.New(),
			Name:         "test-provider",
			ServiceType:  "vm",
			Endpoint:     mockProvider.URL,
			HealthStatus: model.HealthStatusReady,
		}
		Expect(db.Create(&provider).Error).NotTo(HaveOccurred())

		dataStore = store.NewStore(db)
		instanceService = rmsvc.NewInstanceService(dataStore)
		ctx = context.Background()
	})

	AfterEach(func() {
		mockProvider.Close()
		dataStore.Close()
	})

	Describe("CreateInstance", func() {
		It("creates a new instance", func() {
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "test-provider",
				Spec:         map[string]interface{}{"cpu": 2, "memory": "4GB"},
			}

			result, err := instanceService.CreateInstance(ctx, req, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.Id).NotTo(BeNil())
			Expect(result.ProviderName).To(Equal("test-provider"))
			Expect(providerCalled).To(BeTrue())
		})

		It("creates instance with specified ID", func() {
			specifiedID := uuid.New().String()
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "test-provider",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			result, err := instanceService.CreateInstance(ctx, req, &specifiedID)

			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Id).To(Equal(specifiedID))
		})

		It("returns conflict error for duplicate ID", func() {
			specifiedID := uuid.New().String()
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "test-provider",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			// First creation should succeed
			_, err := instanceService.CreateInstance(ctx, req, &specifiedID)
			Expect(err).NotTo(HaveOccurred())

			// Second creation with same ID should fail
			_, err = instanceService.CreateInstance(ctx, req, &specifiedID)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeConflict))
		})

		It("returns not found error for non-existent provider", func() {
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "non-existent-provider",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			_, err := instanceService.CreateInstance(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})

		It("returns provider error when provider exists but is not ready", func() {
			// Create a provider with HealthStatus = NotReady
			notReadyProvider := model.Provider{
				ID:           uuid.New(),
				Name:         "not-ready-provider",
				ServiceType:  "vm",
				Endpoint:     mockProvider.URL,
				HealthStatus: model.HealthStatusNotReady,
			}
			Expect(db.Create(&notReadyProvider).Error).NotTo(HaveOccurred())

			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "not-ready-provider",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			_, err := instanceService.CreateInstance(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeProviderError))
			Expect(svcErr.Message).To(ContainSubstring("not in ready state"))
		})

		It("returns validation error for invalid ID format", func() {
			invalidID := "not-a-uuid"
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "test-provider",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			_, err := instanceService.CreateInstance(ctx, req, &invalidID)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeValidation))
		})

		It("returns provider error when provider endpoint fails", func() {
			// Create a provider with a bad endpoint
			badProvider := model.Provider{
				ID:          uuid.New(),
				Name:        "bad-provider",
				ServiceType: "vm",
				Endpoint:    "http://localhost:1", // Invalid port
			}
			Expect(db.Create(&badProvider).Error).NotTo(HaveOccurred())

			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "bad-provider",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			_, err := instanceService.CreateInstance(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeProviderError))
		})

		It("returns provider error when provider responds with 4xx HTTP error", func() {
			// Create a mock server that returns 400
			mockProvider4xx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": "bad request"}`))
			}))
			defer mockProvider4xx.Close()

			provider4xx := model.Provider{
				ID:           uuid.New(),
				Name:         "provider-4xx",
				ServiceType:  "vm",
				Endpoint:     mockProvider4xx.URL,
				HealthStatus: model.HealthStatusReady,
			}
			Expect(db.Create(&provider4xx).Error).NotTo(HaveOccurred())

			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "provider-4xx",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			_, err := instanceService.CreateInstance(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeProviderError))
			Expect(svcErr.Message).To(ContainSubstring("provider returned error"))
		})

		It("returns provider error when provider responds with 5xx HTTP error", func() {
			// Create a mock server that returns 500
			mockProvider5xx := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error": "internal server error"}`))
			}))
			defer mockProvider5xx.Close()

			provider5xx := model.Provider{
				ID:           uuid.New(),
				Name:         "provider-5xx",
				ServiceType:  "vm",
				Endpoint:     mockProvider5xx.URL,
				HealthStatus: model.HealthStatusReady,
			}
			Expect(db.Create(&provider5xx).Error).NotTo(HaveOccurred())

			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "provider-5xx",
				Spec:         map[string]interface{}{"cpu": 1},
			}

			_, err := instanceService.CreateInstance(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeProviderError))
			Expect(svcErr.Message).To(ContainSubstring("provider returned error"))
		})

		It("returns internal error with instance ID when DB insert fails", func() {
			var instanceID string
			var providerCallCount int
			mockProviderWithID := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				providerCallCount++
				instanceID = uuid.New().String()

				if providerCallCount == 1 {
					sqlDB, _ := db.DB()
					sqlDB.Close()
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{
					"id":     instanceID,
					"status": "PROVISIONING",
				})
			}))
			defer mockProviderWithID.Close()

			providerWithID := model.Provider{
				ID:           uuid.New(),
				Name:         "provider-db-fail",
				ServiceType:  "vm",
				Endpoint:     mockProviderWithID.URL,
				HealthStatus: model.HealthStatusReady,
			}
			Expect(db.Create(&providerWithID).Error).NotTo(HaveOccurred())

			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "provider-db-fail",
				Spec:         map[string]interface{}{"cpu": 2},
			}

			_, err := instanceService.CreateInstance(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeInternal))
			Expect(svcErr.Message).To(ContainSubstring("failed to create database record"))
			Expect(svcErr.Message).To(ContainSubstring(instanceID))
		})
	})

	Describe("GetInstance", func() {
		It("returns an instance", func() {
			// Create an instance first
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "test-provider",
				Spec:         map[string]interface{}{"cpu": 2},
			}
			created, _ := instanceService.CreateInstance(ctx, req, nil)

			result, err := instanceService.GetInstance(ctx, *created.Id)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(*result.Id).To(Equal(*created.Id))
			Expect(result.ProviderName).To(Equal("test-provider"))
		})

		It("returns not found error for non-existent instance", func() {
			_, err := instanceService.GetInstance(ctx, uuid.New().String())

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})

		It("returns validation error for invalid ID format", func() {
			_, err := instanceService.GetInstance(ctx, "invalid-uuid")

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeValidation))
		})
	})

	Describe("ListInstances", func() {
		It("returns empty list when no instances exist", func() {
			result, err := instanceService.ListInstances(ctx, nil, nil, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(*result.Instances).To(BeEmpty())
		})

		It("returns all instances", func() {
			// Create instances
			for i := 0; i < 3; i++ {
				req := &resource_manager.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": i + 1},
				}
				_, err := instanceService.CreateInstance(ctx, req, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			result, err := instanceService.ListInstances(ctx, nil, nil, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Instances).To(HaveLen(3))
		})

		It("respects max page size", func() {
			// Create 5 instances
			for i := 0; i < 5; i++ {
				req := &resource_manager.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": i + 1},
				}
				_, err := instanceService.CreateInstance(ctx, req, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			maxPageSize := 2
			result, err := instanceService.ListInstances(ctx, nil, &maxPageSize, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Instances).To(HaveLen(2))
		})

		It("filters instances by provider name", func() {
			// Create a second provider
			secondProvider := model.Provider{
				ID:           uuid.New(),
				Name:         "second-provider",
				ServiceType:  "vm",
				Endpoint:     mockProvider.URL,
				HealthStatus: model.HealthStatusReady,
			}
			Expect(db.Create(&secondProvider).Error).NotTo(HaveOccurred())

			// Create instances for different providers
			for i := 0; i < 2; i++ {
				req := &resource_manager.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": i + 1},
				}
				_, err := instanceService.CreateInstance(ctx, req, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			for i := 0; i < 3; i++ {
				req := &resource_manager.ServiceTypeInstance{
					ProviderName: "second-provider",
					Spec:         map[string]interface{}{"cpu": i + 1},
				}
				_, err := instanceService.CreateInstance(ctx, req, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			// Filter by first provider
			filterProvider := "test-provider"
			result, err := instanceService.ListInstances(ctx, &filterProvider, nil, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Instances).To(HaveLen(2))
			for _, inst := range *result.Instances {
				Expect(inst.ProviderName).To(Equal("test-provider"))
			}

			// Filter by second provider
			filterProvider = "second-provider"
			result, err = instanceService.ListInstances(ctx, &filterProvider, nil, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Instances).To(HaveLen(3))
			for _, inst := range *result.Instances {
				Expect(inst.ProviderName).To(Equal("second-provider"))
			}
		})

		It("returns next page token when there are more results", func() {
			// Create more instances than the page size
			for i := 0; i < 5; i++ {
				req := &resource_manager.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": i + 1},
				}
				_, err := instanceService.CreateInstance(ctx, req, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			maxPageSize := 2
			result, err := instanceService.ListInstances(ctx, nil, &maxPageSize, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Instances).To(HaveLen(2))
			Expect(result.NextPageToken).NotTo(BeNil())
			Expect(*result.NextPageToken).NotTo(BeEmpty())
		})

		It("uses page token to fetch next page", func() {
			// Create multiple instances
			for i := 0; i < 5; i++ {
				req := &resource_manager.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": i + 1},
				}
				_, err := instanceService.CreateInstance(ctx, req, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get first page
			maxPageSize := 2
			firstPage, err := instanceService.ListInstances(ctx, nil, &maxPageSize, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*firstPage.Instances).To(HaveLen(2))
			Expect(firstPage.NextPageToken).NotTo(BeNil())

			// Get second page using token
			secondPage, err := instanceService.ListInstances(ctx, nil, &maxPageSize, firstPage.NextPageToken)

			Expect(err).NotTo(HaveOccurred())
			Expect(*secondPage.Instances).To(HaveLen(2))
			Expect(secondPage.NextPageToken).NotTo(BeNil())

			// Verify instances are different
			firstIDs := make(map[string]bool)
			for _, inst := range *firstPage.Instances {
				firstIDs[*inst.Id] = true
			}
			for _, inst := range *secondPage.Instances {
				Expect(firstIDs[*inst.Id]).To(BeFalse(), "Instance should not appear in both pages")
			}
		})

		It("returns no next page token on last page", func() {
			// Create exactly 3 instances
			for i := 0; i < 3; i++ {
				req := &resource_manager.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": i + 1},
				}
				_, err := instanceService.CreateInstance(ctx, req, nil)
				Expect(err).NotTo(HaveOccurred())
			}

			// Get first page with size 2
			maxPageSize := 2
			firstPage, err := instanceService.ListInstances(ctx, nil, &maxPageSize, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(*firstPage.Instances).To(HaveLen(2))
			Expect(firstPage.NextPageToken).NotTo(BeNil())

			// Get second page (last page with 1 item)
			secondPage, err := instanceService.ListInstances(ctx, nil, &maxPageSize, firstPage.NextPageToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(*secondPage.Instances).To(HaveLen(1))
			Expect(secondPage.NextPageToken).To(BeNil())
		})
	})

	Describe("DeleteInstance", func() {
		It("deletes an instance", func() {
			// Create an instance first
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "test-provider",
				Spec:         map[string]interface{}{"cpu": 2},
			}
			created, _ := instanceService.CreateInstance(ctx, req, nil)

			err := instanceService.DeleteInstance(ctx, *created.Id)

			Expect(err).NotTo(HaveOccurred())
			Expect(deleteRequested).To(BeTrue())

			// Verify it's deleted
			_, err = instanceService.GetInstance(ctx, *created.Id)
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})

		It("returns not found error for non-existent instance", func() {
			err := instanceService.DeleteInstance(ctx, uuid.New().String())

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})

		It("returns validation error for invalid ID format", func() {
			err := instanceService.DeleteInstance(ctx, "invalid-uuid")

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeValidation))
		})

		It("continues deletion even if provider is missing", func() {
			// Create an instance
			req := &resource_manager.ServiceTypeInstance{
				ProviderName: "test-provider",
				Spec:         map[string]interface{}{"cpu": 2},
			}
			created, _ := instanceService.CreateInstance(ctx, req, nil)

			// Delete the provider from the database
			Expect(db.Delete(&model.Provider{}, "name = ?", "test-provider").Error).NotTo(HaveOccurred())

			// Delete should still succeed (provider gone, but instance deletion continues)
			err := instanceService.DeleteInstance(ctx, *created.Id)

			Expect(err).NotTo(HaveOccurred())

			// Verify it's deleted
			_, err = instanceService.GetInstance(ctx, *created.Id)
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})
	})
})
