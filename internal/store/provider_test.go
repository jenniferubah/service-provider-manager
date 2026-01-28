package store_test

import (
	"context"

	"github.com/dcm-project/service-provider-manager/internal/store"
	"github.com/dcm-project/service-provider-manager/internal/store/model"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ = Describe("Provider Store", func() {
	var (
		db            *gorm.DB
		providerStore store.Provider
		ctx           context.Context
	)

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.Provider{})).To(Succeed())

		providerStore = store.NewProvider(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	})

	Describe("Create", func() {
		It("persists the provider", func() {
			p := newProvider("create-test")
			created, err := providerStore.Create(ctx, p)

			Expect(err).NotTo(HaveOccurred())
			Expect(created.ID).To(Equal(p.ID))
			Expect(created.Name).To(Equal("create-test"))
			Expect(created.SchemaVersion).To(Equal("v1alpha1"))
		})

		It("rejects duplicate names", func() {
			p1 := newProvider("duplicate-name")
			_, err := providerStore.Create(ctx, p1)
			Expect(err).NotTo(HaveOccurred())

			p2 := newProvider("duplicate-name")
			_, err = providerStore.Create(ctx, p2)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Get", func() {
		It("retrieves by ID", func() {
			p := newProvider("get-test")
			providerStore.Create(ctx, p)

			found, err := providerStore.Get(ctx, p.ID)

			Expect(err).NotTo(HaveOccurred())
			Expect(found.Name).To(Equal("get-test"))
		})

		It("returns ErrProviderNotFound for missing ID", func() {
			_, err := providerStore.Get(ctx, uuid.New())

			Expect(err).To(Equal(store.ErrProviderNotFound))
		})
	})

	Describe("GetByName", func() {
		It("retrieves by name", func() {
			p := newProvider("named-provider")
			providerStore.Create(ctx, p)

			found, err := providerStore.GetByName(ctx, "named-provider")

			Expect(err).NotTo(HaveOccurred())
			Expect(found.ID).To(Equal(p.ID))
		})

		It("returns ErrProviderNotFound for missing name", func() {
			_, err := providerStore.GetByName(ctx, "non-existent")

			Expect(err).To(Equal(store.ErrProviderNotFound))
		})
	})

	Describe("List", func() {
		It("returns all providers when filter is nil", func() {
			providerStore.Create(ctx, newProvider("p1"))
			providerStore.Create(ctx, newProvider("p2"))

			providers, err := providerStore.List(ctx, nil, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(providers).To(HaveLen(2))
		})

		It("filters by service type", func() {
			p1 := newProvider("vm-provider")
			p1.ServiceType = "vm"
			providerStore.Create(ctx, p1)

			p2 := newProvider("container-provider")
			p2.ServiceType = "container"
			providerStore.Create(ctx, p2)

			vmType := "vm"
			vms, err := providerStore.List(ctx, &store.ProviderFilter{ServiceType: &vmType}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(vms).To(HaveLen(1))
			Expect(vms[0].Name).To(Equal("vm-provider"))
		})

		It("filters by name", func() {
			providerStore.Create(ctx, newProvider("find-me"))
			providerStore.Create(ctx, newProvider("not-me"))

			name := "find-me"
			providers, err := providerStore.List(ctx, &store.ProviderFilter{Name: &name}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0].Name).To(Equal("find-me"))
		})

		It("filters by both name and service type", func() {
			p1 := newProvider("vm-one")
			p1.ServiceType = "vm"
			providerStore.Create(ctx, p1)

			p2 := newProvider("vm-two")
			p2.ServiceType = "vm"
			providerStore.Create(ctx, p2)

			name := "vm-one"
			vmType := "vm"
			providers, err := providerStore.List(ctx, &store.ProviderFilter{Name: &name, ServiceType: &vmType}, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(providers).To(HaveLen(1))
			Expect(providers[0].Name).To(Equal("vm-one"))
		})

		It("respects pagination limit", func() {
			providerStore.Create(ctx, newProvider("page-p1"))
			providerStore.Create(ctx, newProvider("page-p2"))
			providerStore.Create(ctx, newProvider("page-p3"))

			providers, err := providerStore.List(ctx, nil, &store.Pagination{Limit: 2, Offset: 0})

			Expect(err).NotTo(HaveOccurred())
			Expect(providers).To(HaveLen(2))
		})

		It("respects pagination offset", func() {
			providerStore.Create(ctx, newProvider("offset-p1"))
			providerStore.Create(ctx, newProvider("offset-p2"))
			providerStore.Create(ctx, newProvider("offset-p3"))

			providers, err := providerStore.List(ctx, nil, &store.Pagination{Limit: 10, Offset: 2})

			Expect(err).NotTo(HaveOccurred())
			Expect(providers).To(HaveLen(1))
		})
	})

	Describe("Count", func() {
		It("returns total count without filter", func() {
			providerStore.Create(ctx, newProvider("count-p1"))
			providerStore.Create(ctx, newProvider("count-p2"))

			count, err := providerStore.Count(ctx, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(int64(2)))
		})

		It("returns filtered count", func() {
			p1 := newProvider("count-vm")
			p1.ServiceType = "vm"
			providerStore.Create(ctx, p1)

			p2 := newProvider("count-container")
			p2.ServiceType = "container"
			providerStore.Create(ctx, p2)

			vmType := "vm"
			count, err := providerStore.Count(ctx, &store.ProviderFilter{ServiceType: &vmType})

			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(int64(1)))
		})
	})

	Describe("Delete", func() {
		It("removes the provider", func() {
			p := newProvider("to-delete")
			providerStore.Create(ctx, p)

			err := providerStore.Delete(ctx, p.ID)

			Expect(err).NotTo(HaveOccurred())
		})

		It("returns ErrProviderNotFound for missing ID", func() {
			err := providerStore.Delete(ctx, uuid.New())

			Expect(err).To(Equal(store.ErrProviderNotFound))
		})
	})

	Describe("Update", func() {
		It("modifies existing provider", func() {
			p := newProvider("to-update")
			providerStore.Create(ctx, p)

			p.Endpoint = "https://new-endpoint.com"
			updated, err := providerStore.Update(ctx, p)

			Expect(err).NotTo(HaveOccurred())
			Expect(updated.Endpoint).To(Equal("https://new-endpoint.com"))
		})

		It("returns ErrProviderNotFound for non-existing provider", func() {
			p := newProvider("non-existing")
			_, err := providerStore.Update(ctx, p)

			Expect(err).To(Equal(store.ErrProviderNotFound))
		})
	})
})

func newProvider(name string) model.Provider {
	return model.Provider{
		ID:            uuid.New(),
		Name:          name,
		ServiceType:   "vm",
		SchemaVersion: "v1alpha1",
		Endpoint:      "https://example.com/api",
	}
}
