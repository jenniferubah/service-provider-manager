package service_test

import (
	"context"

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

	Describe("RegisterProvider", func() {
		It("creates a new provider", func() {
			req := newProvider("new-provider")

			resp, err := providerService.RegisterProvider(ctx, req, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Status).NotTo(BeNil())
			Expect(*resp.Status).To(Equal(server.Registered))
			Expect(resp.Name).To(Equal("new-provider"))
		})

		It("updates existing provider with same name and ID", func() {
			req := newProvider("update-test")
			resp1, _ := providerService.RegisterProvider(ctx, req, nil)

			// Re-register with same ID
			req.Id = resp1.Id
			req.Endpoint = "https://updated.example.com"
			resp2, err := providerService.RegisterProvider(ctx, req, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp2.Status).NotTo(BeNil())
			Expect(*resp2.Status).To(Equal(server.Updated))
		})

		It("updates existing provider with same name and no ID (idempotent)", func() {
			req := newProvider("idempotent-test")
			resp1, _ := providerService.RegisterProvider(ctx, req, nil)

			// Re-register with same name but NO ID
			req2 := newProvider("idempotent-test")
			req2.Endpoint = "https://updated.example.com"
			resp2, err := providerService.RegisterProvider(ctx, req2, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(resp2.Status).NotTo(BeNil())
			Expect(*resp2.Status).To(Equal(server.Updated))
			Expect(resp2.Id.String()).To(Equal(resp1.Id.String())) // Same ID returned
			Expect(resp2.Endpoint).To(Equal("https://updated.example.com"))
		})

		It("returns conflict when name exists with different ID", func() {
			req := newProvider("conflict-name")
			providerService.RegisterProvider(ctx, req, nil)

			// Try with different ID
			newID := openapi_types.UUID(uuid.New())
			req.Id = &newID
			_, err := providerService.RegisterProvider(ctx, req, nil)

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeConflict))
		})

		It("returns conflict when providerID exists with different name", func() {
			req := newProvider("first-name")
			resp, _ := providerService.RegisterProvider(ctx, req, nil)

			// Try with same ID but different name
			req2 := newProvider("second-name")
			req2.Id = resp.Id
			_, err := providerService.RegisterProvider(ctx, req2, nil)

			Expect(err).To(HaveOccurred())
			svcErr, ok := err.(*service.ServiceError)
			Expect(ok).To(BeTrue())
			Expect(svcErr.Code).To(Equal(service.ErrCodeConflict))
		})
	})

	Describe("GetProvider", func() {
		It("returns the provider", func() {
			req := newProvider("get-test")
			resp, _ := providerService.RegisterProvider(ctx, req, nil)

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
			providerService.RegisterProvider(ctx, newProvider("p1"), nil)
			providerService.RegisterProvider(ctx, newProvider("p2"), nil)

			providers, err := providerService.ListProviders(ctx, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(providers).To(HaveLen(2))
		})

		It("filters by service type", func() {
			req1 := newProvider("vm-provider")
			req1.ServiceType = "vm"
			providerService.RegisterProvider(ctx, req1, nil)

			req2 := newProvider("container-provider")
			req2.ServiceType = "container"
			providerService.RegisterProvider(ctx, req2, nil)

			providers, err := providerService.ListProviders(ctx, "vm")

			Expect(err).NotTo(HaveOccurred())
			Expect(providers).To(HaveLen(1))
		})
	})

	Describe("UpdateProvider", func() {
		It("updates the provider", func() {
			req := newProvider("update-provider")
			resp, _ := providerService.RegisterProvider(ctx, req, nil)

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
			providerService.RegisterProvider(ctx, newProvider("original-name"), nil)
			resp2, _ := providerService.RegisterProvider(ctx, newProvider("to-rename"), nil)

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
			resp, _ := providerService.RegisterProvider(ctx, req, nil)

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
