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
	"sigs.k8s.io/kind/pkg/cluster"
)

type WorkerTestSuite struct {
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
	resources            []*entropyv1beta1.Resource
}

func (s *WorkerTestSuite) SetupTest() {
	s.ctx, s.moduleClient, s.resourceClient, s.appConfig, s.pool, s.resource, s.kubeProvider, s.cancelModuleClient, s.cancelResourceClient, s.cancel = testbench.SetupTests(s.T(), false)

	modules, err := s.moduleClient.ListModules(s.ctx, &entropyv1beta1.ListModulesRequest{})
	s.Require().NoError(err)
	s.Require().Equal(6, len(modules.GetModules()))

	resources, err := s.resourceClient.ListResources(s.ctx, &entropyv1beta1.ListResourcesRequest{
		Kind: "kubernetes",
	})
	s.Require().NoError(err)
	s.Require().Equal(3, len(resources.GetResources()))
	s.resources = resources.GetResources()

}

func (s *WorkerTestSuite) TestWorkerDefault() {
	testbench.SetupWorker(s.T(), s.ctx, *s.appConfig)

	s.Run("running worker with default config will run one worker that takes any job", func() {
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

	s.Run("running worker with default config will run one worker that takes any job", func() {
		resourceConfig, err := getFirehoseResourceRequest()
		s.Require().NoError(err)

		resourceConfig.Project = s.resources[1].Project
		resourceConfig.Spec.Dependencies = []*entropyv1beta1.ResourceDependency{
			{
				Key:   "kube_cluster",
				Value: s.resources[1].Urn,
			},
		}

		resp, err := s.resourceClient.CreateResource(s.ctx, &entropyv1beta1.CreateResourceRequest{
			Resource: resourceConfig,
		})
		s.Require().NoError(err)

		pods, err := getRunningFirehosePods(s.ctx, s.kubeProvider, testbench.TestClusterName, testbench.TestNamespace, map[string]string{}, 90*time.Second)
		s.Require().NoError(err)
		s.Require().Equal(2, len(pods))

		createdFirehose, err := s.resourceClient.GetResource(s.ctx, &entropyv1beta1.GetResourceRequest{
			Urn: resp.GetResource().Urn,
		})
		s.Require().NoError(err)
		s.Require().NotNil(createdFirehose)
		s.Require().Equal(resource.StatusCompleted, createdFirehose.Resource.State.Status.String())
	})
}

func (s *WorkerTestSuite) TestWorkerScope() {
	projectScope := []string{s.resources[0].Project}
	workerConfig := cli.WorkerConfig{
		Count: 1,
		Scope: map[string][]string{
			"project": projectScope,
		},
	}

	s.appConfig.Syncer.Workers = map[string]cli.WorkerConfig{"test-project-0-worker": workerConfig}
	testbench.SetupWorker(s.T(), s.ctx, *s.appConfig)

	s.Run("running worker with project scoped config will run worker(s) that takes configured project job", func() {
		resourceConfig, err := getFirehoseResourceRequest()
		s.Require().NoError(err)

		resourceConfig.Project = s.resources[0].Project
		resourceConfig.Spec.Dependencies = []*entropyv1beta1.ResourceDependency{
			{
				Key:   "kube_cluster",
				Value: s.resources[0].Urn,
			},
		}

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

	s.Run("running worker with project scoped config will run worker(s) that won't takes none configured project job", func() {
		resourceConfig, err := getFirehoseResourceRequest()
		s.Require().NoError(err)

		resourceConfig.Project = s.resources[1].Project
		resourceConfig.Spec.Dependencies = []*entropyv1beta1.ResourceDependency{
			{
				Key:   "kube_cluster",
				Value: s.resources[1].Urn,
			},
		}

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
		s.Require().Equal(resource.StatusPending, createdFirehose.Resource.State.Status.String())
	})
}

func (s *WorkerTestSuite) TearDownTest() {
	if err := s.pool.Purge(s.resource); err != nil {
		s.T().Fatal(err)
	}

	if err := s.kubeProvider.Delete(testbench.TestClusterName, ""); err != nil {
		s.T().Fatal(err)
	}

	s.cancel()
}

func TestWorkerTestSuite(t *testing.T) {
	suite.Run(t, new(WorkerTestSuite))
}
