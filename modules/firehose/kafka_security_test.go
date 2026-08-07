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

// the injected consumer config matches the reference fixture. Unlike dagger,
// the mount descriptors are not part of it — they travel through ACLMounts.
func TestBuildSecurityConfigs_OAUTHBEARER_MatchesFixture(t *testing.T) {
	got := buildSecurityConfigs(pocStream, oauthbearerProfile(), "team-x", KafkaSecurity{})

	want := map[string]string{
		"SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS":                   "literalfile",
		"SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS_LITERALFILE_CLASS": defaultLiteralFileConfigProviderClass,
		"SOURCE_KAFKA_CONSUMER_CONFIG_SASL_JAAS_CONFIG":                   "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required;",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SASL_LOGIN_CALLBACK_HANDLER_CLASS":  defaultOauthSaslLoginCallbackHandlerClass,
		"SOURCE_KAFKA_CONSUMER_CONFIG_SASL_MECHANISM":                     "OAUTHBEARER",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SECURITY_PROTOCOL":                  "SASL_SSL",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_PROTOCOL":                       "SSL",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_LOCATION":            "/var/secrets/al-gp-id-s-central-kf/certs/truststore.p12",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD":            "${literalfile:/var/secrets/al-gp-id-s-central-kf/passwords/truststore_password:literal-value}",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_TYPE":                "PKCS12",
	}

	assert.Equal(t, want, got)
}

// the provider / callback handler classes are overridable per deployment.
func TestBuildSecurityConfigs_OverridesClasses(t *testing.T) {
	got := buildSecurityConfigs(pocStream, oauthbearerProfile(), "team-x", KafkaSecurity{
		ConfigProviderClass:           "com.example.LiteralFileConfigProvider",
		SaslLoginCallbackHandlerClass: "com.example.PodLoginCallbackHandler",
	})

	assert.Equal(t, "com.example.LiteralFileConfigProvider", got[keyConsumerConfigProvidersLiteralClass])
	assert.Equal(t, "com.example.PodLoginCallbackHandler", got[keyConsumerSaslLoginCallbackHandler])
}

func TestBuildACLMounts_OAUTHBEARER_MatchesFixture(t *testing.T) {
	mounts := buildACLMounts(pocStream, oauthbearerProfile(), "team-x")

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

// a plaintext stream produces no consumer config and no mounts.
func TestPlaintextStream_NoSecurityWiring(t *testing.T) {
	assert.Nil(t, buildSecurityConfigs(pocStream, nil, "team-x", KafkaSecurity{}))
	assert.Nil(t, buildSecurityConfigs(pocStream, &kafkamod.SecurityProfile{}, "team-x", KafkaSecurity{}))
	assert.Nil(t, buildSecurityConfigs(pocStream, &kafkamod.SecurityProfile{SecurityProtocol: "PLAINTEXT"}, "team-x", KafkaSecurity{}))

	assert.Nil(t, buildACLMounts(pocStream, nil, "team-x"))
	assert.Nil(t, buildACLMounts(pocStream, &kafkamod.SecurityProfile{SecurityProtocol: "PLAINTEXT"}, "team-x"))
}

// brokers are populated from the resolved stream URL when not set, and the
// consumer config is injected into the env variables.
func TestApplyStreamSecurity_PopulatesBrokersAndConfig(t *testing.T) {
	out := kafkamod.Output{URL: "broker-1:9098,broker-2:9098", Security: oauthbearerProfile()}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	exr := module.ExpandedResource{
		Dependencies: map[string]module.ResolvedDependency{
			pocStream: {Kind: kafkamod.Module.Kind, Output: outJSON},
		},
	}
	conf := &Config{
		Team:         "team-x",
		StreamName:   pocStream,
		EnvVariables: map[string]string{},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), exr, conf))

	assert.Equal(t, "broker-1:9098,broker-2:9098", conf.EnvVariables[confKeyKafkaBrokers])
	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyConsumerSecurityProtocol])
	assert.Len(t, conf.ACLMounts, 3)
}

// product (Dex) path: the security profile is inlined on conf.StreamSecurity
// with NO kafka dependency present, and the ACL wiring still fires.
func TestApplyStreamSecurity_InlineProfile_NoDependency(t *testing.T) {
	conf := &Config{
		Team:         "team-x",
		StreamName:   pocStream,
		EnvVariables: map[string]string{},
		StreamSecurity: map[string]*kafkamod.SecurityProfile{
			pocStream: oauthbearerProfile(),
		},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf))

	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyConsumerSecurityProtocol])
	assert.Equal(t, "OAUTHBEARER", conf.EnvVariables[keyConsumerSaslMechanism])
	assert.Len(t, conf.ACLMounts, 3)
}

// the flag (sent as an env variable by Dex) makes the driver fetch the kafka
// resource by URN, and is stripped from the env variables afterwards.
func TestApplyStreamSecurity_FlagFetchesInternally(t *testing.T) {
	out := kafkamod.Output{URL: "127.0.0.1:9098", Security: oauthbearerProfile()}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	var gotURN string
	fd := &firehoseDriver{
		getResource: func(_ context.Context, urn string) (*resource.Resource, error) {
			gotURN = urn
			return &resource.Resource{State: resource.State{Output: outJSON}}, nil
		},
		conf: driverConf{KafkaSecurity: KafkaSecurity{ServiceAccount: "aegis-kafka"}},
	}

	exr := module.ExpandedResource{Resource: resource.Resource{Project: "al-dp-id-s"}}
	conf := &Config{
		Team: "team-x",
		EnvVariables: map[string]string{
			keySourceKafkaName:            pocStream,
			keySourceKafkaSecurityEnabled: "true",
		},
	}

	require.NoError(t, fd.applyStreamSecurity(context.Background(), exr, conf))

	assert.Equal(t, resource.GenerateURN(kafkamod.Module.Kind, "al-dp-id-s", pocStream), gotURN)
	assert.Equal(t, "127.0.0.1:9098", conf.EnvVariables[confKeyKafkaBrokers])
	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyConsumerSecurityProtocol])
	assert.Equal(t, "aegis-kafka", conf.ServiceAccount)

	// the transient flag must never reach the running firehose.
	assert.NotContains(t, conf.EnvVariables, keySourceKafkaSecurityEnabled)
}

// a plaintext firehose (no stream name, no stream_security, no dependency) is
// left untouched — env variables, mounts and service account unchanged.
func TestApplyStreamSecurity_PlaintextFirehose_NoWiring(t *testing.T) {
	conf := &Config{
		Team: "team-x",
		EnvVariables: map[string]string{
			confKeyKafkaBrokers: "localhost:9092",
			confKeyKafkaTopic:   "foo-log",
		},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf))

	assert.Equal(t, map[string]string{
		confKeyKafkaBrokers: "localhost:9092",
		confKeyKafkaTopic:   "foo-log",
	}, conf.EnvVariables)
	assert.Nil(t, conf.ACLMounts)
	assert.Empty(t, conf.ServiceAccount)
}

// an explicit brokers value is not overwritten by the resolved stream URL.
func TestApplyStreamSecurity_KeepsExplicitBrokers(t *testing.T) {
	out := kafkamod.Output{URL: "resolved:9098"}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	exr := module.ExpandedResource{
		Dependencies: map[string]module.ResolvedDependency{
			pocStream: {Kind: kafkamod.Module.Kind, Output: outJSON},
		},
	}
	conf := &Config{
		StreamName:   pocStream,
		EnvVariables: map[string]string{confKeyKafkaBrokers: "explicit:9092"},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), exr, conf))
	assert.Equal(t, "explicit:9092", conf.EnvVariables[confKeyKafkaBrokers])
}

// a stream that loses its security profile has the previously injected keys
// and mounts cleared instead of left behind.
func TestApplyStreamSecurity_ClearsStaleWiring(t *testing.T) {
	conf := &Config{
		StreamName: pocStream,
		EnvVariables: map[string]string{
			keyConsumerSecurityProtocol: "SASL_SSL",
			keyConsumerSaslMechanism:    "OAUTHBEARER",
			confKeyKafkaTopic:           "foo-log",
		},
		ACLMounts: []ACLMount{{Name: "stale", MountPath: "/var/secrets/stale", Type: "secret"}},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf))

	assert.Equal(t, map[string]string{confKeyKafkaTopic: "foo-log"}, conf.EnvVariables)
	assert.Nil(t, conf.ACLMounts)
}

// PLAIN/SCRAM: the JAAS config references credentials through the literalfile
// provider — never inlining the secret values.
func TestBuildSecurityConfigs_PlainScram_NoInlinedSecrets(t *testing.T) {
	sp := &kafkamod.SecurityProfile{
		SecurityProtocol: "SASL_SSL",
		SaslMechanism:    "SCRAM-SHA-512",
		ACLs: map[string]kafkamod.ACLCredentialRef{
			"team-x": {SecretName: "team-x-creds", UsernameKey: "username", PasswordKey: "password"},
		},
	}

	got := buildSecurityConfigs(pocStream, sp, "team-x", KafkaSecurity{})
	jaas := got[keyConsumerSaslJaasConfig]

	assert.Contains(t, jaas, "ScramLoginModule")
	assert.Contains(t, jaas, "${literalfile:/var/secrets/al-gp-id-s-central-kf/credentials/username:literal-value}")
	assert.Contains(t, jaas, "${literalfile:/var/secrets/al-gp-id-s-central-kf/credentials/password:literal-value}")
	assert.NotContains(t, jaas, "team-x-creds")
	assert.Equal(t, "literalfile", got[keyConsumerConfigProviders])

	mounts := buildACLMounts(pocStream, sp, "team-x")
	require.Len(t, mounts, 2)
	assert.Equal(t, ACLMount{
		Name:       "al-gp-id-s-central-kf-team-x-creds",
		MountPath:  "/var/secrets/al-gp-id-s-central-kf/credentials",
		SecretName: "team-x-creds",
		Type:       "secret",
	}, mounts[0])
	assert.Equal(t, "projected", mounts[1].Type)
}
