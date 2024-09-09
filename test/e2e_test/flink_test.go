package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/goto/entropy/cli"
	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
	"github.com/goto/entropy/test/testbench"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/kind/pkg/cluster"
)

type FlinkTestSuite struct {
	suite.Suite
	ctx                  context.Context
	moduleClient         entropyv1beta1.ModuleServiceClient
	resourceClient       entropyv1beta1.ResourceServiceClient
	cancelModuleClient   func()
	cancelResourceClient func()
	cancel               func()
	resource             *dockertest.Resource
	pool                 *dockertest.Pool
	appConfig            *cli.Config
	kubeProvider         *cluster.Provider
}

func (s *FlinkTestSuite) SetupTest() {
	s.ctx, s.moduleClient, s.resourceClient, s.appConfig, s.pool, s.resource, s.kubeProvider, s.cancelModuleClient, s.cancelResourceClient, s.cancel = testbench.SetupTests(s.T(), true, true)

	modules, err := s.moduleClient.ListModules(s.ctx, &entropyv1beta1.ListModulesRequest{})
	s.Require().NoError(err)
	s.Require().Equal(9, len(modules.GetModules()))

	resources, err := s.resourceClient.ListResources(s.ctx, &entropyv1beta1.ListResourcesRequest{
		Kind: "kubernetes",
	})
	s.Require().NoError(err)
	s.Require().Equal(3, len(resources.GetResources()))
}

func (s *FlinkTestSuite) TestFlink() {
	s.Run("create flink module return success", func() {
		moduleData, err := os.ReadFile(testbench.TestDataPath + "module/flink_module.json")
		if err != nil {
			s.T().Fatal(err)
		}

		var moduleConfig *entropyv1beta1.Module
		err = json.Unmarshal(moduleData, &moduleConfig)
		if err != nil {
			s.T().Fatal(err)
		}
		_, err = s.moduleClient.CreateModule(s.ctx, &entropyv1beta1.CreateModuleRequest{
			Module: moduleConfig,
		})
		s.Require().NoError(err)
	})
	/*
		s.Run("create flink with invalid config will return invalid error", func() {
			_, err := s.resourceClient.CreateResource(s.ctx, &entropyv1beta1.CreateResourceRequest{
				Resource: &entropyv1beta1.Resource{
					Name:    "test-flink",
					Project: "test-project",
					Kind:    "flink",
					Spec: &entropyv1beta1.ResourceSpec{
						Configs:      structpb.NewStringValue("{}"),
						Dependencies: []*entropyv1beta1.ResourceDependency{},
					},
				},
			})
			s.Assert().Equal(codes.InvalidArgument, status.Convert(err).Code())
		})
	*/
	s.Run("create flink with right config will return success", func() {
		resourceData, err := os.ReadFile(testbench.TestDataPath + "/resource/flink_resource.json")
		if err != nil {
			s.T().Fatal(err)
		}

		var resourceConfig *entropyv1beta1.Resource
		err = json.Unmarshal(resourceData, &resourceConfig)
		if err != nil {
			s.T().Fatal(err)
		}

		_, err = s.resourceClient.CreateResource(s.ctx, &entropyv1beta1.CreateResourceRequest{
			Resource: resourceConfig,
		})
		s.Require().NoError(err)
	})

	resources, err := s.resourceClient.ListResources(s.ctx, &entropyv1beta1.ListResourcesRequest{
		Kind: "flink",
	})
	s.Require().NoError(err)
	s.Require().Equal(1, len(resources.GetResources()))

	s.Run("update flink with right config will return success", func() {
		resourceData, err := os.ReadFile(testbench.TestDataPath + "/resource/flink_resource.json")
		if err != nil {
			s.T().Fatal(err)
		}

		var resourceConfig *entropyv1beta1.Resource
		err = json.Unmarshal(resourceData, &resourceConfig)
		if err != nil {
			s.T().Fatal(err)
		}

		resourceConfig.Spec.Dependencies = nil

		_, err = s.resourceClient.UpdateResource(s.ctx, &entropyv1beta1.UpdateResourceRequest{
			Urn:     resources.GetResources()[0].Urn,
			NewSpec: resourceConfig.Spec,
		})
		s.Require().NoError(err)
	})
}

func (s *FlinkTestSuite) TearDownTest() {
	if err := s.pool.Purge(s.resource); err != nil {
		s.T().Fatal(err)
	}
}

func TestFlinkTestSuite(t *testing.T) {
	suite.Run(t, new(FlinkTestSuite))
}
