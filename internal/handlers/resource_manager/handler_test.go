package resource_manager_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	server "github.com/dcm-project/service-provider-manager/internal/api/server/resource_manager"
	rmhandlers "github.com/dcm-project/service-provider-manager/internal/handlers/resource_manager"
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

var _ = Describe("Resource Manager Handler", func() {
	var (
		db             *gorm.DB
		handler        *rmhandlers.Handler
		ctx            context.Context
		mockProvider   *httptest.Server
		providerCalled bool
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
		mockProvider = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		dataStore := store.NewStore(db)
		instanceService := rmsvc.NewInstanceService(dataStore)
		handler = rmhandlers.NewHandler(instanceService)
		ctx = context.Background()
	})

	AfterEach(func() {
		mockProvider.Close()
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	Describe("GetHealth", func() {
		It("returns ok", func() {
			resp, err := handler.GetHealth(ctx, server.GetHealthRequestObject{})

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.GetHealth200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Status).To(Equal("ok"))
		})
	})

	Describe("CreateInstance", func() {
		It("creates and returns 201", func() {
			req := server.CreateInstanceRequestObject{
				Body: &server.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": 2, "memory": "4GB"},
				},
			}

			resp, err := handler.CreateInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.CreateInstance201JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.ProviderName).To(Equal("test-provider"))
			Expect(jsonResp.Id).NotTo(BeNil())
			Expect(providerCalled).To(BeTrue())
		})

		It("creates with specified ID", func() {
			specifiedID := uuid.New().String()
			req := server.CreateInstanceRequestObject{
				Params: server.CreateInstanceParams{Id: &specifiedID},
				Body: &server.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": 1},
				},
			}

			resp, err := handler.CreateInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.CreateInstance201JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Id).To(Equal(specifiedID))
		})

		It("returns 409 for duplicate ID", func() {
			specifiedID := uuid.New().String()
			req := server.CreateInstanceRequestObject{
				Params: server.CreateInstanceParams{Id: &specifiedID},
				Body: &server.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": 1},
				},
			}

			// First creation should succeed
			resp1, err := handler.CreateInstance(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			_, ok := resp1.(server.CreateInstance201JSONResponse)
			Expect(ok).To(BeTrue())

			// Second creation with same ID should fail
			resp2, err := handler.CreateInstance(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			_, ok = resp2.(server.CreateInstance409ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 400 for non-existent provider", func() {
			req := server.CreateInstanceRequestObject{
				Body: &server.ServiceTypeInstance{
					ProviderName: "non-existent-provider",
					Spec:         map[string]interface{}{"cpu": 1},
				},
			}

			resp, err := handler.CreateInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.CreateInstance404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("GetInstance", func() {
		It("returns instance", func() {
			// Create an instance first
			createReq := server.CreateInstanceRequestObject{
				Body: &server.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": 2},
				},
			}
			createResp, _ := handler.CreateInstance(ctx, createReq)
			created := createResp.(server.CreateInstance201JSONResponse)

			req := server.GetInstanceRequestObject{
				InstanceId: *created.Id,
			}

			resp, err := handler.GetInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.GetInstance200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.ProviderName).To(Equal("test-provider"))
		})

		It("returns 404 for non-existent instance", func() {
			req := server.GetInstanceRequestObject{
				InstanceId: uuid.New().String(),
			}

			resp, err := handler.GetInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.GetInstance404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 400 for invalid ID format", func() {
			req := server.GetInstanceRequestObject{
				InstanceId: "not-a-uuid",
			}

			resp, err := handler.GetInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.GetInstance400ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("ListInstances", func() {
		It("returns empty list initially", func() {
			req := server.ListInstancesRequestObject{}

			resp, err := handler.ListInstances(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListInstances200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Instances).To(BeEmpty())
		})

		It("returns instances", func() {
			// Create instances first
			for i := 0; i < 3; i++ {
				createReq := server.CreateInstanceRequestObject{
					Body: &server.ServiceTypeInstance{
						ProviderName: "test-provider",
						Spec:         map[string]interface{}{"cpu": i + 1},
					},
				}
				_, err := handler.CreateInstance(ctx, createReq)
				Expect(err).NotTo(HaveOccurred())
			}

			resp, err := handler.ListInstances(ctx, server.ListInstancesRequestObject{})

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListInstances200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Instances).To(HaveLen(3))
		})

		It("respects max page size", func() {
			// Create 5 instances
			for i := 0; i < 5; i++ {
				createReq := server.CreateInstanceRequestObject{
					Body: &server.ServiceTypeInstance{
						ProviderName: "test-provider",
						Spec:         map[string]interface{}{"cpu": i + 1},
					},
				}
				_, err := handler.CreateInstance(ctx, createReq)
				Expect(err).NotTo(HaveOccurred())
			}

			maxPageSize := 2
			req := server.ListInstancesRequestObject{
				Params: server.ListInstancesParams{MaxPageSize: &maxPageSize},
			}

			resp, err := handler.ListInstances(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListInstances200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Instances).To(HaveLen(2))
		})
	})

	Describe("DeleteInstance", func() {
		It("deletes instance and returns 204", func() {
			// Create an instance first
			createReq := server.CreateInstanceRequestObject{
				Body: &server.ServiceTypeInstance{
					ProviderName: "test-provider",
					Spec:         map[string]interface{}{"cpu": 2},
				},
			}
			createResp, _ := handler.CreateInstance(ctx, createReq)
			created := createResp.(server.CreateInstance201JSONResponse)

			req := server.DeleteInstanceRequestObject{
				InstanceId: *created.Id,
			}

			resp, err := handler.DeleteInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteInstance204Response)
			Expect(ok).To(BeTrue())

			// Verify it's deleted
			getResp, _ := handler.GetInstance(ctx, server.GetInstanceRequestObject{InstanceId: *created.Id})
			_, ok = getResp.(server.GetInstance404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 404 for non-existent instance", func() {
			req := server.DeleteInstanceRequestObject{
				InstanceId: uuid.New().String(),
			}

			resp, err := handler.DeleteInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteInstance404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 400 for invalid ID format", func() {
			req := server.DeleteInstanceRequestObject{
				InstanceId: "invalid-uuid",
			}

			resp, err := handler.DeleteInstance(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteInstance400ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})
})
