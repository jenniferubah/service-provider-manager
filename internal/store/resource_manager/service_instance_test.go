package store_test

import (
	"context"
	"encoding/json"

	"github.com/dcm-project/service-provider-manager/internal/store/model"
	rmstore "github.com/dcm-project/service-provider-manager/internal/store/resource_manager"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newServiceTypeInstance(providerName, instanceName string, spec any) model.ServiceTypeInstance {
	jsonSpec, _ := json.Marshal(spec)
	return model.ServiceTypeInstance{
		ID:           uuid.New(),
		ProviderName: providerName,
		Status:       "PROVISIONING",
		InstanceName: instanceName,
		Spec:         jsonSpec,
	}
}

var (
	kubevirtProvider = "kubevirt-sp"
)

var _ = Describe("ServiceTypeInstance Store", func() {
	var (
		db  *gorm.DB
		s   rmstore.ServiceTypeInstance
		ctx context.Context
	)

	addInstanceToStore := func(instance model.ServiceTypeInstance) *model.ServiceTypeInstance {
		created, err := s.Create(ctx, instance)
		Expect(err).NotTo(HaveOccurred())
		return created
	}

	BeforeEach(func() {
		var err error
		db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(db.AutoMigrate(&model.ServiceTypeInstance{})).To(Succeed())

		s = rmstore.NewServiceTypeInstance(db)
		ctx = context.Background()
	})

	AfterEach(func() {
		sqlDB, err := db.DB()
		Expect(err).NotTo(HaveOccurred())
		Expect(sqlDB.Close()).To(Succeed())
	})

	Describe("Create", func() {
		It("persists the instance", func() {
			instance := newServiceTypeInstance(
				kubevirtProvider,
				"instance-1",
				map[string]any{"cpu": 2})
			created, err := s.Create(ctx, instance)

			Expect(err).NotTo(HaveOccurred())
			Expect(created.ID).To(Equal(instance.ID))
		})
	})

	Describe("Get", func() {
		It("retrieves by ID", func() {
			seeded := newServiceTypeInstance(kubevirtProvider, "get-inst", map[string]any{"cpu": 1})
			addInstanceToStore(seeded)

			found, err := s.Get(ctx, seeded.ID)

			Expect(err).NotTo(HaveOccurred())
			Expect(found).NotTo(BeNil())
			Expect(found.ProviderName).To(Equal(kubevirtProvider))
			Expect(found.InstanceName).To(Equal("get-inst"))
		})

		It("returns ErrInstanceNotFound for missing ID", func() {
			_, err := s.Get(ctx, uuid.New())
			Expect(err).To(MatchError(rmstore.ErrInstanceNotFound))
		})
	})

	Describe("List", func() {
		BeforeEach(func() {
			addInstanceToStore(newServiceTypeInstance(kubevirtProvider, "instance1", map[string]any{}))
			addInstanceToStore(newServiceTypeInstance(kubevirtProvider, "instance2", map[string]any{}))
			addInstanceToStore(newServiceTypeInstance(kubevirtProvider, "instance3", map[string]any{}))
		})

		It("returns all instances when opts is nil", func() {
			result, err := s.List(ctx, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Instances).To(HaveLen(3))
			Expect(result.NextPageToken).To(BeNil())
		})

		It("filters by provider name", func() {
			result, err := s.List(ctx, &rmstore.ServiceTypeInstanceListOptions{
				ProviderName: &kubevirtProvider,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Instances).To(HaveLen(3))
		})

		It("applies pagination with page size", func() {
			result, err := s.List(ctx, &rmstore.ServiceTypeInstanceListOptions{
				PageSize: 2,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Instances).To(HaveLen(2))
			Expect(result.NextPageToken).NotTo(BeNil())
		})

		It("returns next page using page token", func() {
			// Get first page
			firstPage, err := s.List(ctx, &rmstore.ServiceTypeInstanceListOptions{
				PageSize: 2,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(firstPage.Instances).To(HaveLen(2))
			Expect(firstPage.NextPageToken).NotTo(BeNil())

			// Get second page using token
			secondPage, err := s.List(ctx, &rmstore.ServiceTypeInstanceListOptions{
				PageSize:  2,
				PageToken: firstPage.NextPageToken,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(secondPage.Instances).To(HaveLen(1))
			Expect(secondPage.NextPageToken).To(BeNil())
		})
	})

	Describe("Delete", func() {
		It("removes the instance", func() {
			instance := newServiceTypeInstance(kubevirtProvider, "to-delete", map[string]any{})
			addInstanceToStore(instance)

			Expect(s.Delete(ctx, instance.ID)).To(Succeed())

			_, err := s.Get(ctx, instance.ID)
			Expect(err).To(MatchError(rmstore.ErrInstanceNotFound))
		})

		It("returns ErrInstanceNotFound for missing ID", func() {
			err := s.Delete(ctx, uuid.New())
			Expect(err).To(MatchError(rmstore.ErrInstanceNotFound))
		})
	})

	Describe("ExistsByID", func() {
		It("returns true when instance exists", func() {
			instance := newServiceTypeInstance(kubevirtProvider, "exists", map[string]any{})
			addInstanceToStore(instance)

			exists, err := s.ExistsByID(ctx, instance.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("returns false when instance is missing", func() {
			exists, err := s.ExistsByID(ctx, uuid.New())
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})
	})
})
