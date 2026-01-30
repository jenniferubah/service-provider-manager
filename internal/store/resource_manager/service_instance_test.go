package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dcm-project/service-provider-manager/internal/store/model"
	rmstore "github.com/dcm-project/service-provider-manager/internal/store/resource_manager"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	RegisterTestingT(t)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(db.AutoMigrate(&model.ServiceTypeInstance{})).To(Succeed())
	return db
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	RegisterTestingT(t)
	sqlDB, err := db.DB()
	Expect(err).NotTo(HaveOccurred())
	Expect(sqlDB.Close()).To(Succeed())
}

func newServiceTypeInstance(providerName, serviceType, instanceName string, spec any) model.ServiceTypeInstance {
	jsonSpec, _ := json.Marshal(spec)
	return model.ServiceTypeInstance{
		ID:           uuid.New(),
		ProviderName: providerName,
		ServiceType:  serviceType,
		Status:       "PROVISIONING",
		InstanceName: instanceName,
		Spec:         jsonSpec,
	}
}

func addInstanceToStore(s rmstore.ServiceTypeInstance, ctx context.Context, instance model.ServiceTypeInstance) *model.ServiceTypeInstance {
	created, _ := s.Create(ctx, instance)
	return created
}

var (
	kubevirtProvider = "kubevirt-sp"
	vmServiceType    = "vm"
)

func TestServiceTypeInstanceStore_Create(t *testing.T) {
	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	instance := newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance-1", map[string]any{"cpu": 2})
	created, err := s.Create(ctx, instance)
	Expect(err).NotTo(HaveOccurred())
	Expect(created.ID).To(Equal(instance.ID))
}

func TestServiceTypeInstanceStore_Get(t *testing.T) {
	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	seeded := newServiceTypeInstance(kubevirtProvider, vmServiceType, "get-inst", map[string]any{"cpu": 1})
	addInstanceToStore(s, ctx, seeded)

	cases := []struct {
		name            string
		id              uuid.UUID
		wantErr         error
		wantProvider    string
		wantInstance    string
		wantServiceType string
	}{
		{
			name:            "found",
			id:              seeded.ID,
			wantErr:         nil,
			wantProvider:    kubevirtProvider,
			wantInstance:    "get-inst",
			wantServiceType: vmServiceType,
		},
		{
			name:    "not found",
			id:      uuid.New(),
			wantErr: rmstore.ErrInstanceNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			found, err := s.Get(ctx, tc.id)
			if tc.wantErr != nil {
				Expect(err).To(MatchError(tc.wantErr))
				return
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(found).NotTo(BeNil())
			Expect(found.ProviderName).To(Equal(tc.wantProvider))
			Expect(found.InstanceName).To(Equal(tc.wantInstance))
			Expect(found.ServiceType).To(Equal(tc.wantServiceType))
		})
	}
}

func TestServiceTypeInstanceStore_List_NoFilter(t *testing.T) {

	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	addInstanceToStore(s, ctx, newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance1", map[string]any{}))
	addInstanceToStore(s, ctx, newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance2", map[string]any{}))

	instances, err := s.List(ctx, nil, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(instances).To(HaveLen(2))
}

func TestServiceTypeInstanceStore_List(t *testing.T) {
	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	seed := []model.ServiceTypeInstance{
		newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance1", map[string]any{}),
		newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance2", map[string]any{}),
		newServiceTypeInstance("container-sp", "container", "instance3", map[string]any{}),
	}
	for _, inst := range seed {
		addInstanceToStore(s, ctx, inst)
	}

	cases := []struct {
		name       string
		filter     *rmstore.ServiceTypeInstanceFilter
		pagination *rmstore.Pagination
		wantLen    int
	}{
		{
			name:    "no filter",
			filter:  nil,
			wantLen: 3,
		},
		{
			name:    "filter by provider name",
			filter:  &rmstore.ServiceTypeInstanceFilter{ProviderName: &kubevirtProvider},
			wantLen: 2,
		},
		{
			name:    "filter by service type",
			filter:  &rmstore.ServiceTypeInstanceFilter{ServiceType: &vmServiceType},
			wantLen: 2,
		},
		{
			name:       "pagination limit",
			filter:     nil,
			pagination: &rmstore.Pagination{Limit: 2, Offset: 0},
			wantLen:    2,
		},
		{
			name:       "pagination offset",
			filter:     nil,
			pagination: &rmstore.Pagination{Limit: 10, Offset: 2},
			wantLen:    1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			instances, err := s.List(ctx, tc.filter, tc.pagination)
			Expect(err).NotTo(HaveOccurred())
			Expect(instances).To(HaveLen(tc.wantLen))
		})
	}
}

func TestServiceTypeInstanceStore_List_Pagination(t *testing.T) {
	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	addInstanceToStore(s, ctx, newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance1", map[string]any{}))
	addInstanceToStore(s, ctx, newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance2", map[string]any{}))
	addInstanceToStore(s, ctx, newServiceTypeInstance(kubevirtProvider, vmServiceType, "instance3", map[string]any{}))

	firstTwo, err := s.List(ctx, nil, &rmstore.Pagination{Limit: 2, Offset: 0})
	Expect(err).NotTo(HaveOccurred())
	Expect(firstTwo).To(HaveLen(2))

	lastOne, err := s.List(ctx, nil, &rmstore.Pagination{Limit: 10, Offset: 2})
	Expect(err).NotTo(HaveOccurred())
	Expect(lastOne).To(HaveLen(1))
}

func TestServiceTypeInstanceStore_Delete(t *testing.T) {
	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	instance := newServiceTypeInstance(kubevirtProvider, vmServiceType, "to-delete", map[string]any{})
	addInstanceToStore(s, ctx, instance)

	Expect(s.Delete(ctx, instance.ID)).To(Succeed())

	_, err := s.Get(ctx, instance.ID)
	Expect(err).To(MatchError(rmstore.ErrInstanceNotFound))
}

func TestServiceTypeInstanceStore_Delete_NotFound(t *testing.T) {
	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	err := s.Delete(ctx, uuid.New())
	Expect(err).To(MatchError(rmstore.ErrInstanceNotFound))
}

func TestServiceTypeInstanceStore_ExistsByID(t *testing.T) {
	db := newTestDB(t)
	t.Cleanup(func() { closeDB(t, db) })

	s := rmstore.NewServiceTypeInstance(db)
	ctx := context.Background()

	instance := newServiceTypeInstance(kubevirtProvider, vmServiceType, "exists", map[string]any{})
	addInstanceToStore(s, ctx, instance)

	cases := []struct {
		name    string
		id      uuid.UUID
		want    bool
		wantErr error
	}{
		{name: "exists", id: instance.ID, want: true, wantErr: nil},
		{name: "missing", id: uuid.New(), want: false, wantErr: rmstore.ErrInstanceNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exists, err := s.ExistsByID(ctx, tc.id)
			if tc.wantErr != nil {
				Expect(err).To(MatchError(tc.wantErr))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(exists).To(Equal(tc.want))
		})
	}
}
