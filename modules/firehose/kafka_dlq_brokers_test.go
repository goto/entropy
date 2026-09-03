package firehose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	kafkamod "github.com/goto/entropy/modules/kafka"
)

func TestApplyKafkaDLQBrokers(t *testing.T) {
	t.Parallel()

	outJSON, err := json.Marshal(kafkamod.Output{URL: "ods-kafka-products-dagstream.p.gojek.com:6668"})
	require.NoError(t, err)

	t.Run("fills brokers from orn:entropy:kafka:<project>:dagstream", func(t *testing.T) {
		var gotURN string
		fd := &firehoseDriver{
			getResource: func(_ context.Context, urn string) (*resource.Resource, error) {
				gotURN = urn
				return &resource.Resource{State: resource.State{Output: outJSON}}, nil
			},
		}
		exr := module.ExpandedResource{Resource: resource.Resource{Project: "gjk-p-acc"}}
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
		}}

		require.NoError(t, fd.applyKafkaDLQBrokers(context.Background(), exr, conf))
		assert.Equal(t, "orn:entropy:kafka:gjk-p-acc:dagstream", gotURN)
		assert.Equal(t, "ods-kafka-products-dagstream.p.gojek.com:6668", conf.EnvVariables[confDLQKafkaBrokers])
	})

	t.Run("honours explicit brokers", func(t *testing.T) {
		fd := &firehoseDriver{
			getResource: func(context.Context, string) (*resource.Resource, error) {
				t.Fatal("should not fetch kafka resource when brokers are set")
				return nil, nil
			},
		}
		exr := module.ExpandedResource{Resource: resource.Resource{Project: "gjk-p-acc"}}
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable:   "true",
			confDLQWriterType:   dlqWriterTypeKafka,
			confDLQKafkaBrokers: "custom-broker:9092",
		}}

		require.NoError(t, fd.applyKafkaDLQBrokers(context.Background(), exr, conf))
		assert.Equal(t, "custom-broker:9092", conf.EnvVariables[confDLQKafkaBrokers])
	})

	t.Run("noops when kafka dlq is disabled", func(t *testing.T) {
		fd := &firehoseDriver{
			getResource: func(context.Context, string) (*resource.Resource, error) {
				t.Fatal("should not fetch kafka resource when kafka dlq is off")
				return nil, nil
			},
		}
		exr := module.ExpandedResource{Resource: resource.Resource{Project: "gjk-p-acc"}}
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "false",
			confDLQWriterType: dlqWriterTypeKafka,
		}}

		require.NoError(t, fd.applyKafkaDLQBrokers(context.Background(), exr, conf))
		assert.Empty(t, conf.EnvVariables[confDLQKafkaBrokers])
	})
}
