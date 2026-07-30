package dagger

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

const pocStream = "al-gp-id-s-central-kf"

func oauthbearerProfile() *kafkamod.SecurityProfile {
	return &kafkamod.SecurityProfile{
		SecurityProtocol:  "SASL_SSL",
		SaslMechanism:     "OAUTHBEARER",
		SSLProtocol:       "SSL",
		SSLTruststoreType: "PKCS12",
		SSLCertSecret:     "kafka-central-cert",
		SSLTruststorePasswordDetails: &kafkamod.SecretKeyRef{
			SecretName: "scp-kafka-ssl-secrets",
			Key:        "truststore_password",
		},
	}
}

// Acceptance criterion #1: the injected additional-configurations block matches
// the reference fixture (compared semantically, order-independent).
func TestBuildAdditionalConfigurations_OAUTHBEARER_MatchesFixture(t *testing.T) {
	got := buildAdditionalConfigurations(pocStream, oauthbearerProfile(), "team-x")

	fixture := `{
      "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS": "literalfile",
      "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS_LITERALFILE_CLASS": "com.gtf.dagger.kafka.configproviders.LiteralFileConfigProvider",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_JAAS_CONFIG": "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required;",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_LOGIN_CALLBACK_HANDLER_CLASS": "io.gtflabs.kafka.security.oauthbearer.kubernetes.PodLoginCallbackHandler",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_MECHANISM": "OAUTHBEARER",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SECURITY_PROTOCOL": "SASL_SSL",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_CERT_SECRET": "kafka-central-cert",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_PROTOCOL": "SSL",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_LOCATION": "/var/secrets/al-gp-id-s-central-kf/certs/truststore.p12",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD": "${literalfile:/var/secrets/al-gp-id-s-central-kf/passwords/truststore_password:literal-value}",
      "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD_DETAILS": { "key": "truststore_password", "secretName": "scp-kafka-ssl-secrets" },
      "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_TYPE": "PKCS12"
    }`

	var want map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(fixture), &want))

	// round-trip got through JSON so nested types compare equal to the fixture.
	gotJSON, err := json.Marshal(got)
	require.NoError(t, err)
	var gotNormalized map[string]interface{}
	require.NoError(t, json.Unmarshal(gotJSON, &gotNormalized))

	assert.Equal(t, want, gotNormalized)
}

// Acceptance criterion #2: the podTemplate mounts match the reference fixture.
func TestBuildACLMounts_OAUTHBEARER_MatchesFixture(t *testing.T) {
	sources := []Source{{SourceKafka: SourceKafka{SourceKafkaName: pocStream}}}
	profiles := map[string]*kafkamod.SecurityProfile{pocStream: oauthbearerProfile()}

	mounts := buildACLMounts(sources, profiles, "team-x")

	require.Len(t, mounts, 3)
	assert.Equal(t, ACLMount{
		Name:       "al-gp-id-s-central-kf-kafka-central-cert",
		MountPath:  "/var/secrets/al-gp-id-s-central-kf/certs",
		SecretName: "kafka-central-cert",
		Type:       "secret",
	}, mounts[0])
	assert.Equal(t, ACLMount{
		Name:       "al-gp-id-s-central-kf-scp-kafka-ssl-secrets",
		MountPath:  "/var/secrets/al-gp-id-s-central-kf/passwords",
		SecretName: "scp-kafka-ssl-secrets",
		Type:       "secret",
	}, mounts[1])
	assert.Equal(t, ACLMount{
		Name:      "kafka-token",
		MountPath: "/var/run/secrets/kafka/serviceaccount",
		Type:      "projected",
	}, mounts[2])
}

// Acceptance criterion #5 / R6: a plaintext source produces no additional config
// and no mounts (byte-for-byte unchanged STREAMS/podTemplate).
func TestPlaintextSource_NoSecurityWiring(t *testing.T) {
	assert.Nil(t, buildAdditionalConfigurations(pocStream, nil, "team-x"))
	assert.Nil(t, buildAdditionalConfigurations(pocStream, &kafkamod.SecurityProfile{}, "team-x"))
	assert.Nil(t, buildAdditionalConfigurations(pocStream, &kafkamod.SecurityProfile{SecurityProtocol: "PLAINTEXT"}, "team-x"))

	sources := []Source{{SourceKafka: SourceKafka{SourceKafkaName: pocStream}}}
	assert.Nil(t, buildACLMounts(sources, map[string]*kafkamod.SecurityProfile{}, "team-x"))
}

// R2: bootstrap servers are populated from the resolved stream URL when not set,
// and the additional configuration is attached to the source.
func TestResolveSourceStreams_PopulatesBootstrapAndConfig(t *testing.T) {
	out := kafkamod.Output{URL: "broker-1:9098,broker-2:9098", Security: oauthbearerProfile()}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	exr := module.ExpandedResource{
		Dependencies: map[string]module.ResolvedDependency{
			pocStream: {Kind: kafkamod.Module.Kind, Output: outJSON},
		},
	}
	conf := &Config{
		Team:   "team-x",
		Source: []Source{{SourceKafka: SourceKafka{SourceKafkaName: pocStream}}},
	}

	profiles, err := (&daggerDriver{}).resolveSourceStreams(context.Background(), exr, conf)
	require.NoError(t, err)

	assert.Equal(t, "broker-1:9098,broker-2:9098", conf.Source[0].SourceKafkaConsumerConfigBootstrapServers)
	assert.NotNil(t, conf.Source[0].SourceKafkaConsumerAdditionalConfigurations)
	assert.Contains(t, profiles, pocStream)
}

// Product (Dex) path: the security profile is inlined on conf.StreamSecurity
// with NO kafka dependency present, and the ACL wiring still fires.
func TestResolveSourceStreams_InlineProfile_NoDependency(t *testing.T) {
	conf := &Config{
		Team:   "team-x",
		Source: []Source{{SourceKafka: SourceKafka{SourceKafkaName: pocStream}}},
		StreamSecurity: map[string]*kafkamod.SecurityProfile{
			pocStream: oauthbearerProfile(),
		},
	}

	profiles, err := (&daggerDriver{}).resolveSourceStreams(context.Background(), module.ExpandedResource{}, conf)
	require.NoError(t, err)

	require.Contains(t, profiles, pocStream)
	assert.NotNil(t, conf.Source[0].SourceKafkaConsumerAdditionalConfigurations)
	assert.Equal(t,
		"SASL_SSL",
		conf.Source[0].SourceKafkaConsumerAdditionalConfigurations[keyConsumerSecurityProtocol])

	mounts := buildACLMounts(conf.Source, profiles, conf.Team)
	require.Len(t, mounts, 3)
}

func TestResolveSourceStreams_FlagFetchesInternally(t *testing.T) {
	out := kafkamod.Output{URL: "11.0.0.1:9098", Security: oauthbearerProfile()}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	var gotURN string
	dd := &daggerDriver{
		getResource: func(_ context.Context, urn string) (*resource.Resource, error) {
			gotURN = urn
			return &resource.Resource{State: resource.State{Output: outJSON}}, nil
		},
	}

	exr := module.ExpandedResource{Resource: resource.Resource{Project: "al-dp-id-s"}}
	conf := &Config{
		Team: "team-x",
		Source: []Source{{SourceKafka: SourceKafka{
			SourceKafkaName:            pocStream,
			SourceKafkaSecurityEnabled: true,
		}}},
	}

	profiles, err := dd.resolveSourceStreams(context.Background(), exr, conf)
	require.NoError(t, err)

	assert.Equal(t, resource.GenerateURN(kafkamod.Module.Kind, "al-dp-id-s", pocStream), gotURN)
	require.Contains(t, profiles, pocStream)
	assert.Equal(t, "11.0.0.1:9098", conf.Source[0].SourceKafkaConsumerConfigBootstrapServers)
	assert.NotNil(t, conf.Source[0].SourceKafkaConsumerAdditionalConfigurations)

	// the flag must be stripped from the STREAMS env var.
	assert.NotContains(t, streamsJSON(conf.Source), "SOURCE_KAFKA_SECURITY_ENABLED")
}

// a full plaintext dagger (no stream_security, no kafka dependency) produces
// no additional configs, no mounts, and no taskmanager SA — STREAMS/podTemplate
// unchanged.
func TestApplyStreamSecurity_PlaintextDagger_NoWiring(t *testing.T) {
	conf := &Config{
		Team:   "team-x",
		Source: []Source{{SourceKafka: SourceKafka{SourceKafkaName: pocStream}}},
	}

	require.NoError(t, (&daggerDriver{}).applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf))

	assert.Nil(t, conf.Source[0].SourceKafkaConsumerAdditionalConfigurations)
	assert.Nil(t, conf.ACLMounts)
	assert.Empty(t, conf.TaskManagerServiceAccount)
}

// an explicit bootstrap servers value on the source is not overwritten.
func TestResolveSourceStreams_KeepsExplicitBootstrap(t *testing.T) {
	out := kafkamod.Output{URL: "resolved:9098"}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	exr := module.ExpandedResource{
		Dependencies: map[string]module.ResolvedDependency{
			pocStream: {Kind: kafkamod.Module.Kind, Output: outJSON},
		},
	}
	conf := &Config{
		Source: []Source{{SourceKafka: SourceKafka{
			SourceKafkaName: pocStream,
			SourceKafkaConsumerConfigBootstrapServers: "explicit:9092",
		}}},
	}

	_, err = (&daggerDriver{}).resolveSourceStreams(context.Background(), exr, conf)
	require.NoError(t, err)
	assert.Equal(t, "explicit:9092", conf.Source[0].SourceKafkaConsumerConfigBootstrapServers)
}

// PLAIN/SCRAM: JAAS config references credentials through the literalfile
// provider — never inlining the secret values.
func TestBuildAdditionalConfigurations_PlainScram_NoInlinedSecrets(t *testing.T) {
	sp := &kafkamod.SecurityProfile{
		SecurityProtocol: "SASL_SSL",
		SaslMechanism:    "SCRAM-SHA-512",
		ACLs: map[string]kafkamod.ACLCredentialRef{
			"team-x": {SecretName: "team-x-creds", UsernameKey: "username", PasswordKey: "password"},
		},
	}

	got := buildAdditionalConfigurations(pocStream, sp, "team-x")
	jaas, _ := got[keyConsumerSaslJaasConfig].(string)

	assert.Contains(t, jaas, "ScramLoginModule")
	assert.Contains(t, jaas, "${literalfile:/var/secrets/al-gp-id-s-central-kf/credentials/username:literal-value}")
	assert.Contains(t, jaas, "${literalfile:/var/secrets/al-gp-id-s-central-kf/credentials/password:literal-value}")
	assert.Equal(t, "literalfile", got[keyConsumerConfigProviders])
}
