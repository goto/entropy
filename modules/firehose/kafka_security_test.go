package firehose

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestBuildDLQSecurityConfigs_OAUTHBEARER(t *testing.T) {
	got := buildDLQSecurityConfigs(oauthbearerProfile(), KafkaSecurity{})

	want := map[string]string{
		"DLQ_KAFKA_SECURITY_PROTOCOL":                 "SASL_SSL",
		"DLQ_KAFKA_SASL_MECHANISM":                    "OAUTHBEARER",
		"DLQ_KAFKA_SASL_JAAS_CONFIG":                  "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required;",
		"DLQ_KAFKA_SASL_LOGIN_CALLBACK_HANDLER_CLASS": defaultOauthSaslLoginCallbackHandlerClass,
		"DLQ_KAFKA_SSL_PROTOCOL":                      "SSL",
		"DLQ_KAFKA_SSL_TRUSTSTORE_TYPE":               "PKCS12",
		"DLQ_KAFKA_SSL_TRUSTSTORE_LOCATION":           "/etc/secret/truststore.p12",
	}

	assert.Equal(t, want, got)
}

func TestBuildDLQSecurityConfigs_PlaintextNil(t *testing.T) {
	assert.Nil(t, buildDLQSecurityConfigs(nil, KafkaSecurity{}))
	assert.Nil(t, buildDLQSecurityConfigs(&kafkamod.SecurityProfile{SecurityProtocol: "PLAINTEXT"}, KafkaSecurity{}))
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

func TestKafkaResourceNameFromStreamURN(t *testing.T) {
	assert.Equal(t, "dagstream", kafkaResourceNameFromStreamURN("gjk-p-acc", "gjk-p-acc-dagstream"))
	assert.Equal(t, "dagstream", kafkaResourceNameFromStreamURN("gjk-p-acc", "dagstream"))
	assert.Equal(t, "", kafkaResourceNameFromStreamURN("gjk-p-acc", ""))
}

func TestApplyStreamSecurity_KafkaDLQOnDagstreamPlaintextSource(t *testing.T) {
	plainOut := kafkamod.Output{URL: "source:9092"}
	plainJSON, err := json.Marshal(plainOut)
	require.NoError(t, err)

	dlqOut := kafkamod.Output{URL: "dagstream:9098", Security: oauthbearerProfile()}
	dlqJSON, err := json.Marshal(dlqOut)
	require.NoError(t, err)

	fd := &firehoseDriver{
		getResource: func(_ context.Context, urn string) (*resource.Resource, error) {
			if urn == resource.GenerateURN(kafkamod.Module.Kind, "gjk-p-acc", "dagstream") {
				return &resource.Resource{State: resource.State{Output: dlqJSON}}, nil
			}
			return nil, fmt.Errorf("unexpected urn %s", urn)
		},
		conf: driverConf{KafkaSecurity: KafkaSecurity{ServiceAccount: "aegis-kafka"}},
	}

	exr := module.ExpandedResource{
		Resource: resource.Resource{Project: "gjk-p-acc"},
		Dependencies: map[string]module.ResolvedDependency{
			"central-kf": {Kind: kafkamod.Module.Kind, Output: plainJSON},
		},
	}
	conf := &Config{
		Team:       "team-x",
		StreamName: "central-kf",
		EnvVariables: map[string]string{
			confKeyKafkaBrokers: "source:9092",
			confDLQSinkEnable:   "true",
			confDLQWriterType:   dlqWriterTypeKafka,
			confDLQKafkaTopic:   "app-firehose-dlq",
			confDLQKafkaStream:  "gjk-p-acc-dagstream",
			confDLQKafkaBrokers: "dagstream:9098",
		},
	}

	require.NoError(t, fd.applyStreamSecurity(context.Background(), exr, conf))

	assert.Empty(t, conf.EnvVariables[keyConsumerSecurityProtocol])
	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyDLQSecurityProtocol])
	assert.Equal(t, "dagstream:9098", conf.EnvVariables[keyDLQKafkaBrokers])
	require.NotNil(t, conf.ACL)
	assert.True(t, conf.ACL.KafkaTokenEnabled)
	assert.Equal(t, "aegis-kafka", conf.ServiceAccount)
}

func TestApplyStreamSecurity_KafkaDLQWiresProducerSecurity(t *testing.T) {
	out := kafkamod.Output{URL: "broker-1:9098,broker-2:9098", Security: oauthbearerProfile()}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	exr := module.ExpandedResource{
		Dependencies: map[string]module.ResolvedDependency{
			pocStream: {Kind: kafkamod.Module.Kind, Output: outJSON},
		},
	}
	conf := &Config{
		Team:       "team-x",
		StreamName: pocStream,
		EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: dlqWriterTypeKafka,
			confDLQKafkaTopic: "poc-firehose-dlq",
			// Dex may have pointed DLQ at dagstream; ACL same-stream wiring overwrites.
			keyDLQKafkaBrokers: "dagstream-broker:9092",
		},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), exr, conf))

	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyDLQSecurityProtocol])
	assert.Equal(t, "OAUTHBEARER", conf.EnvVariables[keyDLQSaslMechanism])
	assert.Equal(t, "/etc/secret/truststore.p12", conf.EnvVariables[keyDLQSSLTruststoreLocation])
	assert.Equal(t, "broker-1:9098,broker-2:9098", conf.EnvVariables[keyDLQKafkaBrokers])
	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyConsumerSecurityProtocol])
}

func TestApplyStreamSecurity_BlobDLQSkipsProducerSecurity(t *testing.T) {
	out := kafkamod.Output{URL: "broker-1:9098", Security: oauthbearerProfile()}
	outJSON, err := json.Marshal(out)
	require.NoError(t, err)

	exr := module.ExpandedResource{
		Dependencies: map[string]module.ResolvedDependency{
			pocStream: {Kind: kafkamod.Module.Kind, Output: outJSON},
		},
	}
	conf := &Config{
		Team:       "team-x",
		StreamName: pocStream,
		EnvVariables: map[string]string{
			confDLQSinkEnable: "true",
			confDLQWriterType: "BLOB_STORAGE",
		},
	}

	require.NoError(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), exr, conf))

	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyConsumerSecurityProtocol])
	assert.Empty(t, conf.EnvVariables[keyDLQSecurityProtocol])
	assert.Empty(t, conf.EnvVariables[keyDLQKafkaBrokers])
}

// product (Dex) path: the security profile is inlined on conf.StreamSecurity
// with NO kafka dependency present, and the ACL wiring still fires.
func TestApplyStreamSecurity_InlineProfile_NoDependency(t *testing.T) {
	conf := &Config{
		Team:       "team-x",
		StreamName: pocStream,
		// an inline profile carries no url, so brokers come from the payload.
		EnvVariables: map[string]string{confKeyKafkaBrokers: "broker-1:9098"},
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

// the flag makes the driver fetch the kafka resource by URN, with no dependency
// declared — the Dex product path.
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
		Team:                  "team-x",
		StreamName:            pocStream,
		StreamSecurityEnabled: true,
		EnvVariables:          map[string]string{},
	}

	require.NoError(t, fd.applyStreamSecurity(context.Background(), exr, conf))

	assert.Equal(t, resource.GenerateURN(kafkamod.Module.Kind, "al-dp-id-s", pocStream), gotURN)
	assert.Equal(t, "127.0.0.1:9098", conf.EnvVariables[confKeyKafkaBrokers])
	assert.Equal(t, "SASL_SSL", conf.EnvVariables[keyConsumerSecurityProtocol])
	assert.Equal(t, "aegis-kafka", conf.ServiceAccount)
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

// naming a stream relaxes the schema's brokers requirement, so a stream that
// resolves to nothing must fail the plan rather than deploy without brokers.
func TestApplyStreamSecurity_MissingBrokersFails(t *testing.T) {
	// the stream resolves, but carries no url and the payload has no brokers.
	outJSON, err := json.Marshal(kafkamod.Output{})
	require.NoError(t, err)
	fd := &firehoseDriver{
		getResource: func(context.Context, string) (*resource.Resource, error) {
			return &resource.Resource{State: resource.State{Output: outJSON}}, nil
		},
	}
	conf := &Config{
		StreamName:            pocStream,
		StreamSecurityEnabled: true,
		EnvVariables:          map[string]string{confKeyKafkaTopic: "foo-log"},
	}

	err = fd.applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf)
	require.Error(t, err)
	assert.Contains(t, err.Error(), confKeyKafkaBrokers)

	// same through a declared dependency whose output has no url.
	depJSON, err := json.Marshal(kafkamod.Output{Security: oauthbearerProfile()})
	require.NoError(t, err)
	exr := module.ExpandedResource{
		Dependencies: map[string]module.ResolvedDependency{
			pocStream: {Kind: kafkamod.Module.Kind, Output: depJSON},
		},
	}
	conf = &Config{StreamName: pocStream, EnvVariables: map[string]string{}}

	require.Error(t, (&firehoseDriver{}).applyStreamSecurity(context.Background(), exr, conf))
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
		StreamName:            pocStream,
		StreamSecurityEnabled: true,
		EnvVariables: map[string]string{
			keyConsumerSecurityProtocol: "SASL_SSL",
			keyConsumerSaslMechanism:    "SCRAM-SHA-512",
			keyConsumerConfigProviders:  "literalfile",
			"SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS_LITERALFILE_CLASS": "com.gtf.dagger.kafka.configproviders.LiteralFileConfigProvider",
			keyJavaOptions:      "-Xmx1250m " + jaasConfigJavaOpt,
			confKeyKafkaTopic:   "foo-log",
			confKeyKafkaBrokers: "broker-1:9092",
		},
		ACL: &ACLConfig{SSLConfigCredential: "stale"},
	}

	// the stream now resolves without a security profile.
	outJSON, err := json.Marshal(kafkamod.Output{URL: "broker-1:9092"})
	require.NoError(t, err)
	fd := &firehoseDriver{
		getResource: func(context.Context, string) (*resource.Resource, error) {
			return &resource.Resource{State: resource.State{Output: outJSON}}, nil
		},
	}

	require.NoError(t, fd.applyStreamSecurity(context.Background(), module.ExpandedResource{}, conf))

	assert.Equal(t, map[string]string{
		keyJavaOptions:      "-Xmx1250m",
		confKeyKafkaTopic:   "foo-log",
		confKeyKafkaBrokers: "broker-1:9092",
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
		Team:       "team-x",
		StreamName: pocStream,
		EnvVariables: map[string]string{
			keyJavaOptions:      "-Xmx1250m",
			confKeyKafkaBrokers: "broker-1:9092",
		},
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
