package firehose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKedaKafkaScalerAuthentication(t *testing.T) {
	t.Run("GTF ACL stream", func(t *testing.T) {
		name, ok := kedaKafkaScalerAuthentication(Config{
			StreamSecurityEnabled: true,
			StreamName:            "al-gp-id-s-central-kf",
			ACL:                   &ACLConfig{SSLConfigCredential: "stream-cert"},
		})
		assert.True(t, ok)
		assert.Equal(t, "al-gp-id-s-central-kf", name)
	})

	t.Run("ACL without GTF flag", func(t *testing.T) {
		_, ok := kedaKafkaScalerAuthentication(Config{
			StreamName: "mainstream",
			ACL:        &ACLConfig{},
		})
		assert.False(t, ok)
	})

	t.Run("GTF flag without ACL", func(t *testing.T) {
		_, ok := kedaKafkaScalerAuthentication(Config{
			StreamSecurityEnabled: true,
			StreamName:            "mainstream",
		})
		assert.False(t, ok)
	})
}

func TestKeda_updateTriggersMetadata_ACLAuth(t *testing.T) {
	keda := &Keda{
		Triggers: map[string]Trigger{
			"kafka-trigger": {
				Type: KAFKA,
				Metadata: map[string]string{
					"lagThreshold": "1000000",
				},
			},
		},
	}
	conf := Config{
		StreamSecurityEnabled: true,
		StreamName:            "al-gp-id-s-central-kf",
		ACL:                   &ACLConfig{SSLConfigCredential: "stream-cert"},
		EnvVariables: map[string]string{
			confKeyConsumerID:   "my-project-firehose-1",
			confKeyKafkaTopic:   "orders",
			confKeyKafkaBrokers: "kafka.example:9092",
		},
	}

	require.NoError(t, keda.updateTriggersMetadata(conf))

	trigger := keda.Triggers["kafka-trigger"]
	assert.Equal(t, "my-project-firehose-1", trigger.Metadata[KedaKafkaMetadataConsumerGroupKey])
	assert.Equal(t, "orders", trigger.Metadata[KedaKafkaMetadataTopicKey])
	assert.Equal(t, "kafka.example:9092", trigger.Metadata[KedaKafkaMetadataBootstrapServersKey])
	assert.Equal(t, KedaKafkaMetadataTLSEnable, trigger.Metadata[KedaKafkaMetadataTLSKey])
	assert.Equal(t, "al-gp-id-s-central-kf", trigger.AuthenticationRef.Name)
}

func TestKeda_GetHelmValues_includesAuthenticationRef(t *testing.T) {
	keda := &Keda{
		MinReplicas: 1,
		MaxReplicas: 3,
		Triggers: map[string]Trigger{
			"kafka-trigger": {Type: KAFKA, Metadata: map[string]string{"lagThreshold": "1"}},
		},
	}
	conf := Config{
		StreamSecurityEnabled: true,
		StreamName:            "secured-stream",
		Namespace:             "de-firehose-mc",
		ACL:                   &ACLConfig{SSLConfigCredential: "cert"},
		EnvVariables: map[string]string{
			confKeyConsumerID:   "cg-1",
			confKeyKafkaTopic:   "t",
			confKeyKafkaBrokers: "b:9092",
		},
	}

	values, err := keda.GetHelmValues(conf)
	require.NoError(t, err)

	kedaValues, ok := values["triggers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, kedaValues, 1)
	authRef, ok := kedaValues[0]["authenticationRef"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "secured-stream", authRef["name"])
	meta, ok := kedaValues[0]["metadata"].(map[string]string)
	require.True(t, ok)
	assert.Equal(t, KedaKafkaMetadataTLSEnable, meta[KedaKafkaMetadataTLSKey])
}
