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

// the injected consumer config matches odin's: fixed /etc/secret truststore
// location plus the filename the chart selects out of the cert secret, and no
// password (it arrives as a secretKeyRef env var).
func TestBuildSecurityConfigs_OAUTHBEARER_MatchesOdin(t *testing.T) {
	got := buildSecurityConfigs(oauthbearerProfile(), KafkaSecurity{})

	want := map[string]string{
		"SOURCE_KAFKA_CONSUMER_CONFIG_SECURITY_PROTOCOL":                 "SASL_SSL",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SASL_MECHANISM":                    "OAUTHBEARER",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SASL_JAAS_CONFIG":                  "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required;",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SASL_LOGIN_CALLBACK_HANDLER_CLASS": defaultOauthSaslLoginCallbackHandlerClass,
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_PROTOCOL":                      "SSL",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_TYPE":               "PKCS12",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_LOCATION":           "/etc/secret/truststore.p12",
		"SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_FILENAME":           "truststore.p12",
	}

	assert.Equal(t, want, got)
}

// no secret value is ever injected into the config.
func TestBuildSecurityConfigs_NeverInlinesSecrets(t *testing.T) {
	got := buildSecurityConfigs(oauthbearerProfile(), KafkaSecurity{})

	for key, val := range got {
		assert.NotContains(t, val, "scp-kafka-ssl-secrets", "secret name leaked into %s", key)
		assert.NotContains(t, val, "truststore_password", "password key leaked into %s", key)
	}
}

// JKS streams get the .jks extension in both the location and the filename.
func TestBuildSecurityConfigs_JKSTruststore(t *testing.T) {
	sp := oauthbearerProfile()
	sp.SSLTruststoreType = "JKS"

	got := buildSecurityConfigs(sp, KafkaSecurity{})

	assert.Equal(t, "/etc/secret/truststore.jks", got[keyConsumerSSLTruststoreLocation])
	assert.Equal(t, "truststore.jks", got[keyConsumerSSLTruststoreFilename])
}

func TestBuildACLConfig_OAUTHBEARER(t *testing.T) {
	acl := buildACLConfig(pocStream, oauthbearerProfile(), "team-x")

	require.NotNil(t, acl)
	assert.Equal(t, "kafka-central-cert", acl.SSLConfigCredential)
	assert.Equal(t, "truststore.p12", acl.TruststoreFilename)
	assert.Equal(t, &SecretKeyRef{SecretName: "scp-kafka-ssl-secrets", Key: "truststore_password"}, acl.TruststorePassword)
	assert.True(t, acl.KafkaTokenEnabled)
	assert.Empty(t, acl.JaasConfigCredential)
}

// a plaintext stream produces no consumer config and no ACL values.
func TestPlaintextStream_NoSecurityWiring(t *testing.T) {
	assert.Nil(t, buildSecurityConfigs(nil, KafkaSecurity{}))
	assert.Nil(t, buildSecurityConfigs(&kafkamod.SecurityProfile{}, KafkaSecurity{}))
	assert.Nil(t, buildSecurityConfigs(&kafkamod.SecurityProfile{SecurityProtocol: "PLAINTEXT"}, KafkaSecurity{}))

	assert.Nil(t, buildACLConfig(pocStream, nil, "team-x"))
	assert.Nil(t, buildACLConfig(pocStream, &kafkamod.SecurityProfile{SecurityProtocol: "PLAINTEXT"}, "team-x"))
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
	require.NotNil(t, conf.ACL)
	assert.True(t, conf.ACL.KafkaTokenEnabled)
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
	require.NotNil(t, conf.ACL)
	assert.Equal(t, "kafka-central-cert", conf.ACL.SSLConfigCredential)
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
// left untouched — env variables, ACL values and service account unchanged.
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
	assert.Nil(t, conf.ACL)
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

// a stream that loses its security profile has the previously injected keys,
// ACL values and JAAS java option cleared instead of left behind — including
// the config-provider keys written by an older build.
func TestApplyStreamSecurity_ClearsStaleWiring(t *testing.T) {
	conf := &Config{
		StreamName: pocStream,
		EnvVariables: map[string]string{
			keyConsumerSecurityProtocol: "SASL_SSL",
			keyConsumerSaslMechanism:    "SCRAM-SHA-512",
			keyConsumerConfigProviders:  "literalfile",
			"SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS_LITERALFILE_CLASS": "com.gtf.dagger.kafka.configproviders.LiteralFileConfigProvider",
			keyJavaOptions:    "-Xmx1250m " + jaasConfigJavaOpt,
			confKeyKafkaTopic: "foo-log",
		},
		ACL: &ACLConfig{SSLConfigCredential: "stale"},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf))

	assert.Equal(t, map[string]string{
		keyJavaOptions:    "-Xmx1250m",
		confKeyKafkaTopic: "foo-log",
	}, conf.EnvVariables)
	assert.Nil(t, conf.ACL)
}

// SCRAM streams read credentials from a mounted jaas.conf: no JAAS config env
// variable, a jaas secret to mount, and the JVM option pointing at it.
func TestApplyStreamSecurity_ScramUsesJaasFile(t *testing.T) {
	sp := &kafkamod.SecurityProfile{
		SecurityProtocol: "SASL_PLAINTEXT",
		SaslMechanism:    "SCRAM-SHA-512",
		ACLs: map[string]kafkamod.ACLCredentialRef{
			"team-x": {SecretName: "team-x-creds", UsernameKey: "username", PasswordKey: "password"},
		},
	}
	conf := &Config{
		Team:           "team-x",
		StreamName:     pocStream,
		EnvVariables:   map[string]string{keyJavaOptions: "-Xmx1250m"},
		StreamSecurity: map[string]*kafkamod.SecurityProfile{pocStream: sp},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf))

	assert.NotContains(t, conf.EnvVariables, keyConsumerSaslJaasConfig)
	assert.Equal(t, "-Xmx1250m "+jaasConfigJavaOpt, conf.EnvVariables[keyJavaOptions])
	require.NotNil(t, conf.ACL)
	assert.Equal(t, "team-x-creds", conf.ACL.JaasConfigCredential)
	assert.False(t, conf.ACL.KafkaTokenEnabled)
}

// without an explicit credential secret, the jaas secret falls back to odin's
// <team>-<stream>-jaas convention, with underscores normalised to dashes.
func TestJaasSecretName_OdinConvention(t *testing.T) {
	sp := &kafkamod.SecurityProfile{SecurityProtocol: "SASL_PLAINTEXT", SaslMechanism: "SCRAM-SHA-512"}

	assert.Equal(t, "team-x-al-gp-id-s-central-kf-jaas", jaasSecretName(pocStream, sp, "team-x"))
	assert.Equal(t, "team-x-my-stream-jaas", jaasSecretName("my_stream", sp, "team_x"))
	assert.Empty(t, jaasSecretName(pocStream, sp, ""))
}
