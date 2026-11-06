package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/internal/store/postgres"
	"github.com/goto/entropy/pkg/errors"
)

const testParentURN = "orn:entropy:firehose:test-project-00:test-firehose"

type AddonStoreTestSuite struct {
	suite.Suite
	ctx      context.Context
	pool     *dockertest.Pool
	resource *dockertest.Resource
	store    *postgres.Store
}

func (s *AddonStoreTestSuite) SetupTest() {
	s.store, s.pool, s.resource = newTestClient(s.T())
	s.ctx = context.Background()

	if _, err := bootstrapResources(s.ctx, s.store); err != nil {
		s.T().Fatal(err)
	}
}

func (s *AddonStoreTestSuite) TearDownTest() {
	if err := s.pool.Purge(s.resource); err != nil {
		s.T().Fatal(err)
	}
}

func (s *AddonStoreTestSuite) newAddon(parentURN, typ string) *addon.Addon {
	now := time.Now().UTC().Truncate(time.Millisecond)
	return &addon.Addon{
		URN:         addon.GenerateURN(parentURN, typ),
		ParentURN:   parentURN,
		Type:        typ,
		Spec:        json.RawMessage(`{"labels":{"a":"b"}}`),
		Output:      json.RawMessage(`{}`),
		Status:      addon.StatusCompleted,
		SpecVersion: 1,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "alice",
		UpdatedBy:   "alice",
	}
}

func (s *AddonStoreTestSuite) TestCreateGetList() {
	a := s.newAddon(testParentURN, addon.TypeLogs)
	s.Require().NoError(s.store.CreateAddon(s.ctx, a))
	s.NotZero(a.ID)

	got, err := s.store.GetAddon(s.ctx, testParentURN, addon.TypeLogs)
	s.Require().NoError(err)
	s.Equal(a.ID, got.ID)
	s.Equal(a.URN, got.URN)
	s.JSONEq(string(a.Spec), string(got.Spec))
	s.Equal(int64(1), got.SpecVersion)

	dup := s.newAddon(testParentURN, addon.TypeLogs)
	s.ErrorIs(s.store.CreateAddon(s.ctx, dup), errors.ErrConflict)

	list, err := s.store.ListAddons(s.ctx, testParentURN)
	s.Require().NoError(err)
	s.Len(list, 1)

	list, err = s.store.ListAddons(s.ctx, "orn:entropy:firehose:none:none")
	s.Require().NoError(err)
	s.Empty(list)

	_, err = s.store.GetAddon(s.ctx, testParentURN, addon.TypeWebUI)
	s.ErrorIs(err, errors.ErrNotFound)
}

func (s *AddonStoreTestSuite) TestUpdateChecksVersion() {
	a := s.newAddon(testParentURN, addon.TypeLogs)
	s.Require().NoError(s.store.CreateAddon(s.ctx, a))

	updated := *a
	updated.Spec = json.RawMessage(`{}`)
	updated.Output = json.RawMessage(`{"registration_id":"x"}`)
	updated.SpecVersion = 2
	updated.UpdatedBy = "bob"
	s.Require().NoError(s.store.UpdateAddon(s.ctx, updated, 1))

	got, err := s.store.GetAddon(s.ctx, testParentURN, addon.TypeLogs)
	s.Require().NoError(err)
	s.Equal(int64(2), got.SpecVersion)
	s.Equal("bob", got.UpdatedBy)
	s.JSONEq(`{"registration_id":"x"}`, string(got.Output))

	// stale version.
	s.ErrorIs(s.store.UpdateAddon(s.ctx, updated, 1), errors.ErrConflict)
}

func (s *AddonStoreTestSuite) TestSoftDeleteAndReAdd() {
	first := s.newAddon(testParentURN, addon.TypeLogs)
	s.Require().NoError(s.store.CreateAddon(s.ctx, first))
	s.Require().NoError(s.store.SoftDeleteAddon(s.ctx, first.ID, "bob"))
	s.ErrorIs(s.store.SoftDeleteAddon(s.ctx, first.ID, "bob"), errors.ErrNotFound)

	_, err := s.store.GetAddon(s.ctx, testParentURN, addon.TypeLogs)
	s.ErrorIs(err, errors.ErrNotFound)
	s.ErrorIs(s.store.UpdateAddon(s.ctx, *first, 1), errors.ErrConflict)

	// re-adding inserts a new row; the tombstoned one does not block it.
	second := s.newAddon(testParentURN, addon.TypeLogs)
	s.Require().NoError(s.store.CreateAddon(s.ctx, second))
	s.NotEqual(first.ID, second.ID)

	got, err := s.store.GetAddon(s.ctx, testParentURN, addon.TypeLogs)
	s.Require().NoError(err)
	s.Equal(second.ID, got.ID)
}

func (s *AddonStoreTestSuite) TestResourceSoftDeleteCascades() {
	const otherParentURN = "orn:entropy:dagger:test-project-01:test-dagger"

	logs := s.newAddon(testParentURN, addon.TypeLogs)
	s.Require().NoError(s.store.CreateAddon(s.ctx, logs))
	other := s.newAddon(otherParentURN, addon.TypeWebUI)
	s.Require().NoError(s.store.CreateAddon(s.ctx, other))

	s.Require().NoError(s.store.SoftDelete(s.ctx, testParentURN, "carol"))

	list, err := s.store.ListAddons(s.ctx, testParentURN)
	s.Require().NoError(err)
	s.Empty(list)

	_, err = s.store.GetAddon(s.ctx, otherParentURN, addon.TypeWebUI)
	s.NoError(err)

	// a missing resource still fails and touches nothing.
	s.ErrorIs(s.store.SoftDelete(s.ctx, testParentURN, "carol"), errors.ErrNotFound)
}

func (s *AddonStoreTestSuite) TestConfigs() {
	now := time.Now().UTC().Truncate(time.Millisecond)
	cfg := addon.Config{Project: "team-a", Type: addon.TypeLogs, Configs: json.RawMessage(`{"endpoint":"http://a"}`), CreatedAt: now, UpdatedAt: now}
	s.Require().NoError(s.store.PutAddonConfig(s.ctx, cfg))

	cfg.Configs = json.RawMessage(`{"endpoint":"http://b"}`)
	s.Require().NoError(s.store.PutAddonConfig(s.ctx, cfg))

	got, err := s.store.GetAddonConfig(s.ctx, "team-a", addon.TypeLogs)
	s.Require().NoError(err)
	s.JSONEq(`{"endpoint":"http://b"}`, string(got.Configs))

	s.Require().NoError(s.store.DeleteAddonConfig(s.ctx, "team-a", addon.TypeLogs))
	s.ErrorIs(s.store.DeleteAddonConfig(s.ctx, "team-a", addon.TypeLogs), errors.ErrNotFound)
	_, err = s.store.GetAddonConfig(s.ctx, "team-a", addon.TypeLogs)
	s.ErrorIs(err, errors.ErrNotFound)
}

func TestAddonStore(t *testing.T) {
	suite.Run(t, new(AddonStoreTestSuite))
}
