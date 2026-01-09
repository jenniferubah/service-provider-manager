package handlers_test

import (
	"context"

	"github.com/dcm-project/service-provider-manager/internal/api/server"
	"github.com/dcm-project/service-provider-manager/internal/handlers"
	"github.com/dcm-project/service-provider-manager/internal/service"
	"github.com/dcm-project/service-provider-manager/internal/store"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ = Describe("Handler", func() {
	var (
		db      *gorm.DB
		handler *handlers.Handler
		ctx     context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.Provider{})).To(Succeed())

		dataStore := store.NewStore(db)
		providerService := service.NewProviderService(dataStore)
		handler = handlers.NewHandler(providerService)
		ctx = context.Background()
	})

	AfterEach(func() {
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

	Describe("CreateProvider", func() {
		It("creates and returns 201", func() {
			req := server.CreateProviderRequestObject{
				Body: &server.Provider{
					Name:          "test-provider",
					Endpoint:      "https://example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}

			resp, err := handler.CreateProvider(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.CreateProvider201JSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 200 for idempotent re-registration", func() {
			req := server.CreateProviderRequestObject{
				Body: &server.Provider{
					Name:          "idempotent-provider",
					Endpoint:      "https://example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}

			// First call creates
			resp1, err := handler.CreateProvider(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			_, ok := resp1.(server.CreateProvider201JSONResponse)
			Expect(ok).To(BeTrue())

			// Second call updates (same name, no ID)
			resp2, err := handler.CreateProvider(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			_, ok = resp2.(server.CreateProvider200JSONResponse)
			Expect(ok).To(BeTrue())
		})

		It("returns 409 for name conflict with different ID", func() {
			// Create first provider
			req1 := server.CreateProviderRequestObject{
				Body: &server.Provider{
					Name:          "conflict-name",
					Endpoint:      "https://example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}
			_, err := handler.CreateProvider(ctx, req1)
			Expect(err).NotTo(HaveOccurred())

			// Try to create with same name but different ID
			differentID := openapi_types.UUID(uuid.New())
			req2 := server.CreateProviderRequestObject{
				Params: server.CreateProviderParams{Id: &differentID},
				Body: &server.Provider{
					Name:          "conflict-name",
					Endpoint:      "https://other.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}

			resp, err := handler.CreateProvider(ctx, req2)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.CreateProvider409ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("ListProviders", func() {
		It("returns empty list initially", func() {
			req := server.ListProvidersRequestObject{}

			resp, err := handler.ListProviders(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListProviders200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Providers).To(BeEmpty())
		})

		It("returns providers", func() {
			// Create providers first
			for _, name := range []string{"provider-1", "provider-2"} {
				createReq := server.CreateProviderRequestObject{
					Body: &server.Provider{
						Name:          name,
						Endpoint:      "https://example.com",
						ServiceType:   "vm",
						SchemaVersion: "v1alpha1",
					},
				}
				_, err := handler.CreateProvider(ctx, createReq)
				Expect(err).NotTo(HaveOccurred())
			}

			resp, err := handler.ListProviders(ctx, server.ListProvidersRequestObject{})

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ListProviders200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(*jsonResp.Providers).To(HaveLen(2))
		})
	})

	Describe("GetProvider", func() {
		It("returns provider", func() {
			// Create a provider first
			createReq := server.CreateProviderRequestObject{
				Body: &server.Provider{
					Name:          "get-me",
					Endpoint:      "https://example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}
			createResp, _ := handler.CreateProvider(ctx, createReq)
			created := createResp.(server.CreateProvider201JSONResponse)

			req := server.GetProviderRequestObject{
				ProviderId: *created.Id,
			}

			resp, err := handler.GetProvider(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.GetProvider200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.Name).To(Equal("get-me"))
		})

		It("returns 404 for non-existent provider", func() {
			req := server.GetProviderRequestObject{
				ProviderId: openapi_types.UUID(uuid.New()),
			}

			resp, err := handler.GetProvider(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.GetProvider404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("ApplyProvider", func() {
		It("updates existing provider", func() {
			// Create a provider first
			createReq := server.CreateProviderRequestObject{
				Body: &server.Provider{
					Name:          "to-update",
					Endpoint:      "https://example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}
			createResp, _ := handler.CreateProvider(ctx, createReq)
			created := createResp.(server.CreateProvider201JSONResponse)

			// Update it
			updateReq := server.ApplyProviderRequestObject{
				ProviderId: *created.Id,
				Body: &server.Provider{
					Id:            created.Id,
					Name:          "to-update",
					Endpoint:      "https://updated.example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}

			resp, err := handler.ApplyProvider(ctx, updateReq)

			Expect(err).NotTo(HaveOccurred())
			jsonResp, ok := resp.(server.ApplyProvider200JSONResponse)
			Expect(ok).To(BeTrue())
			Expect(jsonResp.Endpoint).To(Equal("https://updated.example.com"))
		})

		It("returns 404 for non-existent provider", func() {
			req := server.ApplyProviderRequestObject{
				ProviderId: openapi_types.UUID(uuid.New()),
				Body: &server.Provider{
					Name:          "test",
					Endpoint:      "https://example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}

			resp, err := handler.ApplyProvider(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.ApplyProvider404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})

	Describe("DeleteProvider", func() {
		It("deletes provider and returns 204", func() {
			// Create a provider first
			createReq := server.CreateProviderRequestObject{
				Body: &server.Provider{
					Name:          "to-delete",
					Endpoint:      "https://example.com",
					ServiceType:   "vm",
					SchemaVersion: "v1alpha1",
				},
			}
			createResp, _ := handler.CreateProvider(ctx, createReq)
			created := createResp.(server.CreateProvider201JSONResponse)

			req := server.DeleteProviderRequestObject{
				ProviderId: *created.Id,
			}

			resp, err := handler.DeleteProvider(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteProvider204Response)
			Expect(ok).To(BeTrue())
		})

		It("returns 404 for non-existent provider", func() {
			req := server.DeleteProviderRequestObject{
				ProviderId: openapi_types.UUID(uuid.New()),
			}

			resp, err := handler.DeleteProvider(ctx, req)

			Expect(err).NotTo(HaveOccurred())
			_, ok := resp.(server.DeleteProvider404ApplicationProblemPlusJSONResponse)
			Expect(ok).To(BeTrue())
		})
	})
})
