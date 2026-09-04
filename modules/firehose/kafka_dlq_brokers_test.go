package firehose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/resource"
)

func TestResolveKafkaDLQTopic(t *testing.T) {
	t.Parallel()

	t.Run("renders module default from firehose name", func(t *testing.T) {
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
			confDLQKafkaTopic: "{{ .name }}-firehose-dlq",
		}}
		res := resource.Resource{Name: "orders", URN: "orn:entropy:firehose:test-project:orders"}

		require.NoError(t, resolveKafkaDLQTopic(res, conf))
		assert.Equal(t, "orders-firehose-dlq", conf.EnvVariables[confDLQKafkaTopic])
	})

	t.Run("keeps explicit topic", func(t *testing.T) {
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
			confDLQKafkaTopic: "legacy-custom-dlq",
		}}

		require.NoError(t, resolveKafkaDLQTopic(resource.Resource{Name: "orders"}, conf))
		assert.Equal(t, "legacy-custom-dlq", conf.EnvVariables[confDLQKafkaTopic])
	})

	t.Run("noops when kafka dlq is disabled", func(t *testing.T) {
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "false",
			confDLQKafkaTopic: "{{ .name }}-firehose-dlq",
		}}

		require.NoError(t, resolveKafkaDLQTopic(resource.Resource{Name: "orders"}, conf))
		assert.Equal(t, "{{ .name }}-firehose-dlq", conf.EnvVariables[confDLQKafkaTopic])
	})
}

func TestReplaceLegacySharedDLQTopic(t *testing.T) {
	t.Parallel()

	fd := &firehoseDriver{conf: driverConf{EnvVariables: map[string]string{
		confDLQKafkaTopic: "{{ .name }}-firehose-dlq",
	}}}

	t.Run("replaces shared retry topic on first kafka dlq enable", func(t *testing.T) {
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
			confDLQKafkaTopic: legacySharedDLQKafkaTopic,
		}}
		fd.replaceLegacySharedDLQTopic(conf, false)
		assert.Equal(t, "{{ .name }}-firehose-dlq", conf.EnvVariables[confDLQKafkaTopic])
	})

	t.Run("keeps shared retry topic if kafka dlq was already enabled", func(t *testing.T) {
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
			confDLQKafkaTopic: legacySharedDLQKafkaTopic,
		}}
		fd.replaceLegacySharedDLQTopic(conf, true)
		assert.Equal(t, legacySharedDLQKafkaTopic, conf.EnvVariables[confDLQKafkaTopic])
	})

	t.Run("keeps custom topic on first enable", func(t *testing.T) {
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
			confDLQKafkaTopic: "legacy-custom-dlq",
		}}
		fd.replaceLegacySharedDLQTopic(conf, false)
		assert.Equal(t, "legacy-custom-dlq", conf.EnvVariables[confDLQKafkaTopic])
	})

	t.Run("leaves shared retry topic when module default is not a template", func(t *testing.T) {
		withoutTemplate := &firehoseDriver{conf: driverConf{EnvVariables: map[string]string{
			confDLQKafkaTopic: legacySharedDLQKafkaTopic,
		}}}
		conf := &Config{EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
			confDLQKafkaTopic: legacySharedDLQKafkaTopic,
		}}
		withoutTemplate.replaceLegacySharedDLQTopic(conf, false)
		assert.Equal(t, legacySharedDLQKafkaTopic, conf.EnvVariables[confDLQKafkaTopic])
	})
}

func TestDropRemovedKafkaDLQEnv(t *testing.T) {
	t.Parallel()

	conf := &Config{EnvVariables: map[string]string{
		confDLQKafkaTopicCreate:    "true",
		confDLQKafkaTopicRetention: "604800",
		confDLQKafkaTopic:          "orders-firehose-dlq",
	}}
	dropRemovedKafkaDLQEnv(conf)
	_, hasCreate := conf.EnvVariables[confDLQKafkaTopicCreate]
	_, hasRetention := conf.EnvVariables[confDLQKafkaTopicRetention]
	assert.False(t, hasCreate)
	assert.False(t, hasRetention)
	assert.Equal(t, "orders-firehose-dlq", conf.EnvVariables[confDLQKafkaTopic])
}
