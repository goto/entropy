package postgres_test

import (
	"cmp"
	"context"
	"slices"
	"testing"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/internal/store/postgres"
	"github.com/goto/entropy/pkg/errors"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"
)

type ResourceStoreTestSuite struct {
	suite.Suite
	ctx       context.Context
	pool      *dockertest.Pool
	resource  *dockertest.Resource
	store     *postgres.Store
	resources []resource.Resource
}

func (s *ResourceStoreTestSuite) SetupTest() {
	s.store, s.pool, s.resource = newTestClient(s.T())
	s.ctx = context.Background()

	var err error
	s.resources, err = bootstrapResources(s.ctx, s.store)
	if err != nil {
		s.T().Fatal(err)
	}

	s.Assert().Equal(len(s.resources), 6)
}

func (s *ResourceStoreTestSuite) TestSyncOne() {
	type testCase struct {
		Description string
		Scope       map[string][]string
		syncFn      resource.SyncFn
		ErrString   string
	}

	projectScope := []string{"test-project-00"}
	kindScope := []string{"dagger"}
	unknownScope := []string{"unknown"}

	testCases := []testCase{
		{
			Description: "if scope is empty, it will take any job",
			Scope:       map[string][]string{},
			syncFn: func(ctx context.Context, res resource.Resource) (*resource.Resource, error) {
				if res.URN == "" {
					return nil, errors.New("no empty resource")
				}
				return &res, nil
			},
		},
		{
			Description: "take job with project test-project-00",
			Scope: map[string][]string{
				"project": projectScope,
			},
			syncFn: func(ctx context.Context, res resource.Resource) (*resource.Resource, error) {
				if res.Project != "test-project-00" {
					return nil, errors.New("wrong resource project")
				}
				return &res, nil
			},
		},
		{
			Description: "take job with kind dagger",
			Scope: map[string][]string{
				"kind": kindScope,
			},
			syncFn: func(ctx context.Context, res resource.Resource) (*resource.Resource, error) {
				if res.Kind != "dagger" {
					return nil, errors.New("wrong resource kind")
				}
				return &res, nil
			},
		},
		{
			Description: "throw error for unknown field",
			Scope: map[string][]string{
				"unknown": unknownScope,
			},
			syncFn: func(ctx context.Context, res resource.Resource) (*resource.Resource, error) {
				return &res, nil
			},
			ErrString: "pq: column \"unknown\" does not exist",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.Description, func() {
			err := s.store.SyncOne(s.ctx, tc.Scope, tc.syncFn)
			if tc.ErrString != "" {
				if err.Error() != tc.ErrString {
					s.T().Fatalf("got error %s, expected was %s", err.Error(), tc.ErrString)
				}
			} else {
				if err != nil {
					s.T().Fatalf("got error %s, expected was none", err.Error())
				}
			}
		})
	}
}

func (s *ResourceStoreTestSuite) TestUpdateLabels() {
	target := s.resources[0]

	newLabels := map[string]string{"description": "test firehose resource"}
	target.Labels = newLabels
	target.UpdatedBy = "test-user"

	err := s.store.UpdateLabels(s.ctx, target, true, "patch:labels")
	s.Assert().NoError(err)

	got, err := s.store.GetByURN(s.ctx, target.URN)
	s.Require().NoError(err)

	s.Assert().Equal(newLabels, got.Labels)
	s.Assert().Equal(s.resources[0].Spec.Configs, got.Spec.Configs)
	s.Assert().Equal(s.resources[0].State.Status, got.State.Status)
	s.Assert().Equal(s.resources[0].Spec.Dependencies, got.Spec.Dependencies)

	revisions, err := s.store.Revisions(s.ctx, resource.RevisionsSelector{URN: target.URN})

	slices.SortFunc(revisions, func(a, b resource.Revision) int {
		return cmp.Compare(a.ID, b.ID)
	})

	s.Require().NoError(err)
	s.Require().NotEmpty(revisions)
	s.Assert().Equal("patch:labels", revisions[len(revisions)-1].Reason)
	s.Assert().Equal(newLabels, revisions[len(revisions)-1].Labels)
}

func (s *ResourceStoreTestSuite) TearDownTest() {
	if err := s.pool.Purge(s.resource); err != nil {
		s.T().Fatal(err)
	}
}

func TestResourceStore(t *testing.T) {
	suite.Run(t, new(ResourceStoreTestSuite))
}
