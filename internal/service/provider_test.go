package service_test

import (
	"context"
	"fmt"

	"github.com/dcm-project/service-provider-manager/internal/api/server"
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

var _ = Describe("ProviderService", func() {
	var (
		db              *gorm.DB
		dataStore       store.Store
		providerService *service.ProviderService
		ctx             context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.Provider{})).To(Succeed())

		dataStore = store.NewStore(db)
		providerService = service.NewProviderService(dataStore)
		ctx = context.Background()
	})

	AfterEach(func() {
		dataStore.Close()
	})

	Describe("RegisterOrUpdateProvider", func() {
		It("creates a new provider", func() {
			req := newProvider("new-provider")

			resp, err := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Status).NotTo(BeNil())
			Expect(*resp.Status).To(Equal(server.Registered))
			Expect(resp.Name).To(Equal("new-provider"))
		})

		It("updates existing provider with same name and ID", func() {
			req := newProvider("update-test")
			resp1, _ := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			// Re-register with same ID
			req.Id = resp1.Id
			req.Endpoint = "https://updated.example.com"
			resp2, err := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp2.Status).NotTo(BeNil())
			Expect(*resp2.Status).To(Equal(server.Updated))
		})

		It("updates existing provider with same name and no ID (idempotent)", func() {
			req := newProvider("idempotent-test")
			resp1, _ := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			// Re-register with same name but NO ID
			req2 := newProvider("idempotent-test")
			req2.Endpoint = "https://updated.example.com"
			resp2, err := providerService.RegisterOrUpdateProvider(ctx, req2, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp2.Status).NotTo(BeNil())
			Expect(*resp2.Status).To(Equal(server.Updated))
			Expect(resp2.Id.String()).To(Equal(resp1.Id.String())) // Same ID returned
			Expect(resp2.Endpoint).To(Equal("https://updated.example.com"))
		})

		It("returns conflict when name exists with different ID", func() {
			req := newProvider("conflict-name")
			providerService.RegisterOrUpdateProvider(ctx, req, nil)

			// Try with different ID
			newID := openapi_types.UUID(uuid.New())
			req.Id = &newID
			_, err := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeConflict))
		})

		It("returns conflict when providerID exists with different name", func() {
			req := newProvider("first-name")
			resp, _ := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			// Try with same ID but different name
			req2 := newProvider("second-name")
			req2.Id = resp.Id
			_, err := providerService.RegisterOrUpdateProvider(ctx, req2, nil)

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeConflict))
		})
	})

	Describe("GetProvider", func() {
		It("returns the provider", func() {
			req := newProvider("get-test")
			resp, _ := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			provider, err := providerService.GetProvider(ctx, resp.Id.String())

			Expect(err).NotTo(HaveOccurred())
			Expect(provider.Name).To(Equal("get-test"))
		})

		It("returns error for non-existent provider", func() {
			_, err := providerService.GetProvider(ctx, uuid.New().String())

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})
	})

	Describe("ListProviders", func() {
		It("returns all providers", func() {
			providerService.RegisterOrUpdateProvider(ctx, newProvider("p1"), nil)
			providerService.RegisterOrUpdateProvider(ctx, newProvider("p2"), nil)

			result, err := providerService.ListProviders(ctx, "", 0, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Providers).To(HaveLen(2))
		})

		It("filters by service type", func() {
			req1 := newProvider("vm-provider")
			req1.ServiceType = "vm"
			providerService.RegisterOrUpdateProvider(ctx, req1, nil)

			req2 := newProvider("container-provider")
			req2.ServiceType = "container"
			providerService.RegisterOrUpdateProvider(ctx, req2, nil)

			result, err := providerService.ListProviders(ctx, "vm", 0, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Providers).To(HaveLen(1))
		})

		It("returns error for negative page size", func() {
			_, err := providerService.ListProviders(ctx, "", -1, "")

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeValidation))
		})

		It("coerces page size to max", func() {
			for i := 0; i < 5; i++ {
				providerService.RegisterOrUpdateProvider(ctx, newProvider(fmt.Sprintf("coerce-p%d", i)), nil)
			}

			result, err := providerService.ListProviders(ctx, "", 2, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Providers).To(HaveLen(2))
			Expect(result.NextPageToken).NotTo(BeEmpty())
		})

		It("paginates through results", func() {
			for i := 0; i < 5; i++ {
				providerService.RegisterOrUpdateProvider(ctx, newProvider(fmt.Sprintf("paginate-p%d", i)), nil)
			}

			// First page
			result1, err := providerService.ListProviders(ctx, "", 2, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(result1.Providers).To(HaveLen(2))
			Expect(result1.NextPageToken).NotTo(BeEmpty())

			// Second page
			result2, err := providerService.ListProviders(ctx, "", 2, result1.NextPageToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(result2.Providers).To(HaveLen(2))
			Expect(result2.NextPageToken).NotTo(BeEmpty())

			// Third page (last)
			result3, err := providerService.ListProviders(ctx, "", 2, result2.NextPageToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(result3.Providers).To(HaveLen(1))
			Expect(result3.NextPageToken).To(BeEmpty())
		})

		It("returns error for invalid page token", func() {
			_, err := providerService.ListProviders(ctx, "", 0, "invalid-token")

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeValidation))
		})
	})

	Describe("UpdateProvider", func() {
		It("updates the provider", func() {
			req := newProvider("update-provider")
			resp, _ := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			update := &server.Provider{
				Id:            resp.Id,
				Name:          "update-provider",
				Endpoint:      "https://updated.example.com",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			}

			updated, err := providerService.UpdateProvider(ctx, resp.Id.String(), update)

			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Endpoint).To(Equal("https://updated.example.com"))
		})

		It("returns conflict when renaming to existing name", func() {
			// Create two providers
			providerService.RegisterOrUpdateProvider(ctx, newProvider("original-name"), nil)
			resp2, _ := providerService.RegisterOrUpdateProvider(ctx, newProvider("to-rename"), nil)

			// Try to rename second provider to first provider's name
			update := &server.Provider{
				Name:          "original-name",
				Endpoint:      "https://example.com",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			}

			_, err := providerService.UpdateProvider(ctx, resp2.Id.String(), update)

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeConflict))
		})

		It("returns error for non-existent provider", func() {
			update := &server.Provider{
				Name:          "test",
				Endpoint:      "https://example.com",
				ServiceType:   "vm",
				SchemaVersion: "v1alpha1",
			}

			_, err := providerService.UpdateProvider(ctx, uuid.New().String(), update)

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})
	})

	Describe("DeleteProvider", func() {
		It("deletes the provider", func() {
			req := newProvider("to-delete")
			resp, _ := providerService.RegisterOrUpdateProvider(ctx, req, nil)

			err := providerService.DeleteProvider(ctx, resp.Id.String())

			Expect(err).NotTo(HaveOccurred())
		})

		It("returns error for non-existent provider", func() {
			err := providerService.DeleteProvider(ctx, uuid.New().String())

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeNotFound))
		})
	})
})

func newProvider(name string) *server.Provider {
	return &server.Provider{
		Name:          name,
		Endpoint:      "https://example.com/api",
		ServiceType:   "vm",
		SchemaVersion: "v1alpha1",
	}
}
