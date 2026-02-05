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
			ID:            uuid.New(),
			Name:          "test-provider",
			ServiceType:   "vm",
			Endpoint:      mockProvider.URL,
			SchemaVersion: "v1alpha1",
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
				ID:            uuid.New(),
				Name:          "bad-provider",
				ServiceType:   "vm",
				Endpoint:      "http://localhost:1", // Invalid port
				SchemaVersion: "v1alpha1",
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
			result, err := instanceService.ListInstances(ctx, nil, nil, "")

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

			result, err := instanceService.ListInstances(ctx, nil, nil, "")

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
			result, err := instanceService.ListInstances(ctx, nil, &maxPageSize, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(*result.Instances).To(HaveLen(2))
		})

		It("returns validation error for invalid page token", func() {
			_, err := instanceService.ListInstances(ctx, nil, nil, "invalid-token")

			Expect(err).To(HaveOccurred())
			var svcErr *service.ServiceError
			Expect(err).To(BeAssignableToTypeOf(svcErr))
			errors.As(err, &svcErr)
			Expect(svcErr.Code).To(Equal(service.ErrCodeValidation))
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
