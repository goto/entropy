package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/goto/entropy/cli"
	"github.com/goto/entropy/core/resource"
	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
	"github.com/goto/entropy/test/testbench"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"sigs.k8s.io/kind/pkg/cluster"
)

type FirehoseTestSuite struct {
	suite.Suite
	ctx                  context.Context
	moduleClient         entropyv1beta1.ModuleServiceClient
	resourceClient       entropyv1beta1.ResourceServiceClient
	cancelResourceClient func()
	cancelModuleClient   func()
	cancel               func()
	appConfig            *cli.Config
	pool                 *dockertest.Pool
	resource             *dockertest.Resource
	kubeProvider         *cluster.Provider
}

func (s *FirehoseTestSuite) SetupTest() {
	s.ctx, s.moduleClient, s.resourceClient, s.appConfig, s.pool, s.resource, s.kubeProvider, s.cancelModuleClient, s.cancelResourceClient, s.cancel = testbench.SetupTests(s.T(), true)

	modules, err := s.moduleClient.ListModules(s.ctx, &entropyv1beta1.ListModulesRequest{})
	s.Require().NoError(err)
	s.Require().Equal(6, len(modules.GetModules()))

	resources, err := s.resourceClient.ListResources(s.ctx, &entropyv1beta1.ListResourcesRequest{
		Kind: "kubernetes",
	})
	s.Require().NoError(err)
	s.Require().Equal(3, len(resources.GetResources()))
}

func (s *FirehoseTestSuite) TestCreateFirehose() {
	s.Run("create firehose with invalid request body should return invalid error", func() {
		_, err := s.resourceClient.CreateResource(s.ctx, &entropyv1beta1.CreateResourceRequest{
			Resource: &entropyv1beta1.Resource{
				Name:    "test-firehose",
				Project: "test-project-0",
				Kind:    "firehose",
				Spec: &entropyv1beta1.ResourceSpec{
					Configs:      structpb.NewStringValue("{}"),
					Dependencies: []*entropyv1beta1.ResourceDependency{},
				},
			},
		})
		s.Assert().Equal(codes.InvalidArgument, status.Convert(err).Code())
	})

	s.Run("create firehose with right request body should return no error and run a new firehose resource", func() {
		resourceConfig, err := getFirehoseResourceRequest()
		s.Require().NoError(err)

		resp, err := s.resourceClient.CreateResource(s.ctx, &entropyv1beta1.CreateResourceRequest{
			Resource: resourceConfig,
		})
		s.Require().NoError(err)

		pods, err := getRunningFirehosePods(s.ctx, s.kubeProvider, testbench.TestClusterName, testbench.TestNamespace, map[string]string{}, 90*time.Second)
		s.Require().NoError(err)
		s.Require().Equal(1, len(pods))

		createdFirehose, err := s.resourceClient.GetResource(s.ctx, &entropyv1beta1.GetResourceRequest{
			Urn: resp.GetResource().Urn,
		})
		s.Require().NoError(err)
		s.Require().NotNil(createdFirehose)
		s.Require().Equal(resource.StatusCompleted, createdFirehose.Resource.State.Status.String())
	})
}

func (s *FirehoseTestSuite) TearDownTest() {
	if err := s.pool.Purge(s.resource); err != nil {
		s.T().Fatal(err)
	}

	if err := s.kubeProvider.Delete(testbench.TestClusterName, ""); err != nil {
		s.T().Fatal(err)
	}

	s.cancel()
}

func TestFirehoseTestSuite(t *testing.T) {
	suite.Run(t, new(FirehoseTestSuite))
}
