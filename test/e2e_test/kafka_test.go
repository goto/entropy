package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
	"github.com/goto/entropy/test/testbench"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type KafkaTestSuite struct {
	suite.Suite
	ctx                  context.Context
	moduleClient         entropyv1beta1.ModuleServiceClient
	resourceClient       entropyv1beta1.ResourceServiceClient
	cancelModuleClient   func()
	cancelResourceClient func()
	cancel               func()
	resource             *dockertest.Resource
	pool                 *dockertest.Pool
}

func (s *KafkaTestSuite) SetupTest() {
	s.ctx, s.moduleClient, s.resourceClient, _, s.pool, s.resource, _, s.cancelModuleClient, s.cancelResourceClient, s.cancel = testbench.SetupTests(s.T(), false, false)
}

func (s *KafkaTestSuite) TestKafka() {
	s.Run("create kafka module return success", func() {
		moduleData, err := os.ReadFile(testbench.TestDataPath + "/module/kafka_module.json")
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

	s.Run("create kafka with invalid config will return invalid error", func() {
		_, err := s.resourceClient.CreateResource(s.ctx, &entropyv1beta1.CreateResourceRequest{
			Resource: &entropyv1beta1.Resource{
				Name:    "test-kafka",
				Project: "test-project",
				Kind:    "kafka",
				Spec: &entropyv1beta1.ResourceSpec{
					Configs:      structpb.NewStringValue("{}"),
					Dependencies: []*entropyv1beta1.ResourceDependency{},
				},
			},
		})
		s.Assert().Equal(codes.InvalidArgument, status.Convert(err).Code())
	})

	s.Run("create kafka with right config will return success", func() {
		resourceData, err := os.ReadFile(testbench.TestDataPath + "/resource/kafka_resource.json")
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
		Kind: "kafka",
	})
	s.Require().NoError(err)
	s.Require().Equal(1, len(resources.GetResources()))

	s.Run("update kafka with right config will return success", func() {
		resourceData, err := os.ReadFile(testbench.TestDataPath + "/resource/kafka_resource.json")
		if err != nil {
			s.T().Fatal(err)
		}

		var resourceConfig *entropyv1beta1.Resource
		err = json.Unmarshal(resourceData, &resourceConfig)
		if err != nil {
			s.T().Fatal(err)
		}

		_, err = s.resourceClient.UpdateResource(s.ctx, &entropyv1beta1.UpdateResourceRequest{
			Urn:     resources.GetResources()[0].Urn,
			NewSpec: resourceConfig.Spec,
		})
		s.Require().NoError(err)
	})
}

func (s *KafkaTestSuite) TearDownTest() {
	if err := s.pool.Purge(s.resource); err != nil {
		s.T().Fatal(err)
	}
}

func TestKafkaTestSuite(t *testing.T) {
	suite.Run(t, new(KafkaTestSuite))
}
