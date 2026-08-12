package firehose

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	kafkamod "github.com/goto/entropy/modules/kafka"
)

// SASL/SSL consumer config keys. Firehose passes every
// SOURCE_KAFKA_CONSUMER_CONFIG_* env variable straight to the kafka consumer,
// so they are flat env variables here. The key set and the values mirror odin's
// firehose adapter (app/firehose/adapter.js on gtf-master), which is the
// behaviour being migrated onto entropy.
const (
	keyConsumerSecurityProtocol         = "SOURCE_KAFKA_CONSUMER_CONFIG_SECURITY_PROTOCOL"
	keyConsumerSaslMechanism            = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_MECHANISM"
	keyConsumerSaslJaasConfig           = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_JAAS_CONFIG"
	keyConsumerSaslLoginCallbackHandler = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_LOGIN_CALLBACK_HANDLER_CLASS"
	keyConsumerSSLProtocol              = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_PROTOCOL"
	keyConsumerSSLTruststoreType        = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_TYPE"
	keyConsumerSSLTruststoreLocation    = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_LOCATION"
	keyConsumerSSLTruststoreFilename    = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_FILENAME"

	// keyJavaOptions carries the JAAS file location for SCRAM/PLAIN streams.
	// It is user-owned, so only the JAAS option itself is added or removed.
	keyJavaOptions = "_JAVA_OPTIONS"
)

// legacy keys from the config-provider approach. They are no longer emitted but
// are still swept, so resources planned by an older build do not keep a
// dangling provider reference.
const (
	keyConsumerSSLTruststorePassword     = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD"
	keyConsumerConfigProviders           = "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS"
	keyConsumerConfigProviderClassPrefix = "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS_"
	keyConsumerConfigProviderClassSuffix = "_CLASS"
)

// transient inputs Dex may send as env variables. They describe the stream to
// resolve and are never forwarded to the running firehose.
const (
	keySourceKafkaName            = "SOURCE_KAFKA_NAME"
	keySourceKafkaSecurityEnabled = "SOURCE_KAFKA_SECURITY_ENABLED"
)

// managedSecurityKeys are owned by this module: they are wiped and rebuilt on
// every plan of a firehose that names a kafka stream, so a stream that loses
// its ACLs does not leave stale configuration behind.
var managedSecurityKeys = []string{
	keyConsumerSecurityProtocol,
	keyConsumerSaslMechanism,
	keyConsumerSaslJaasConfig,
	keyConsumerSaslLoginCallbackHandler,
	keyConsumerSSLProtocol,
	keyConsumerSSLTruststoreType,
	keyConsumerSSLTruststoreLocation,
	keyConsumerSSLTruststoreFilename,
	keyConsumerSSLTruststorePassword,
	keyConsumerConfigProviders,
}

const (
	oauthConsumerSaslJaasConfig = "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required;"

	// the OAUTHBEARER login callback handler shipped by the platform kafka
	// security library. Override per deployment when the firehose image
	// packages it under a different name.
	defaultOauthSaslLoginCallbackHandlerClass = "io.gtflabs.kafka.security.oauthbearer.kubernetes.PodLoginCallbackHandler"
)

const (
	securityProtocolSASLSSL       = "SASL_SSL"
	securityProtocolSASLPlaintext = "SASL_PLAINTEXT"
	securityProtocolSSL           = "SSL"
	saslMechanismOauthbearer      = "OAUTHBEARER"
	saslMechanismPlain            = "PLAIN"
	saslMechanismScram            = "SCRAM-SHA-512"
	truststoreTypePKCS12          = "PKCS12"
)

// Mount layout, matching odin's firehose manifest. The chart mounts the stream's
// cert secret at /etc/secret and the JAAS secret at /etc/secret/kafka; the
// projected kafka service-account token lands at kafkaTokenMountPath.
const (
	secretMountPath = "/etc/secret"
	// the cert secret gets its own subdirectory rather than odin's bare
	// /etc/secret: the firehose chart already mounts its own secret there for
	// sink credentials, and two volumes cannot share a mount path.
	certMountPath       = secretMountPath + "/kafka-cert"
	jaasSecretMountPath = secretMountPath + "/kafka"
	jaasConfigFileName  = "jaas.conf"
	jaasConfigJavaOpt   = "-Djava.security.auth.login.config=" + jaasSecretMountPath + "/" + jaasConfigFileName
	truststoreFileBase  = "truststore"
	jaasSecretSuffix    = "jaas"

	kafkaTokenMountPath = "/var/run/secrets/kafka/serviceaccount"
)

// KafkaSecurity carries the deployment level knobs for ACL streams.
type KafkaSecurity struct {
	// SaslLoginCallbackHandlerClass is the OAUTHBEARER login callback handler
	// that exchanges the projected service-account token for a kafka token.
	SaslLoginCallbackHandlerClass string `json:"sasl_login_callback_handler_class,omitempty"`

	// ServiceAccount is the OAuth identity authorized for ACL streams. Used
	// when the resource config does not set one. Empty preserves the chart's
	// default service account.
	ServiceAccount string `json:"service_account,omitempty"`
}

func (k KafkaSecurity) withDefaults() KafkaSecurity {
	if k.SaslLoginCallbackHandlerClass == "" {
		k.SaslLoginCallbackHandlerClass = defaultOauthSaslLoginCallbackHandlerClass
	}
	return k
}

func isOauthbearerStream(sp *kafkamod.SecurityProfile) bool {
	return sp != nil &&
		sp.SecurityProtocol == securityProtocolSASLSSL &&
		sp.SaslMechanism == saslMechanismOauthbearer &&
		sp.SSLCertSecret != "" &&
		sp.SSLTruststorePasswordDetails != nil
}

func isPlainOrScramStream(sp *kafkamod.SecurityProfile) bool {
	if sp == nil {
		return false
	}
	protoOK := sp.SecurityProtocol == securityProtocolSASLPlaintext || sp.SecurityProtocol == securityProtocolSASLSSL
	mechOK := sp.SaslMechanism == saslMechanismPlain || sp.SaslMechanism == saslMechanismScram
	return protoOK && mechOK
}

// usesSSLMaterial reports whether the stream presents a truststore. odin keys
// this off the security protocol containing "SSL", covering both SSL and
// SASL_SSL.
func usesSSLMaterial(sp *kafkamod.SecurityProfile) bool {
	return sp != nil && strings.Contains(sp.SecurityProtocol, securityProtocolSSL)
}

// hasSecurityProfile reports whether the profile requires any SASL/SSL wiring.
func hasSecurityProfile(sp *kafkamod.SecurityProfile) bool {
	return sp != nil && sp.SecurityProtocol != "" && !strings.EqualFold(sp.SecurityProtocol, "PLAINTEXT")
}

// truststoreFileName is the file name the truststore is projected as, and also
// the key it is read from inside the cert secret.
func truststoreFileName(truststoreType string) string {
	if strings.EqualFold(truststoreType, truststoreTypePKCS12) {
		return truststoreFileBase + ".p12"
	}
	return truststoreFileBase + ".jks"
}

// buildSecurityConfigs builds the SOURCE_KAFKA_CONSUMER_CONFIG_* env variables
// for the source stream. Returns nil for plaintext streams so env variables
// stay unchanged.
//
// No secret value is ever placed here: the truststore password reaches the
// container as a secretKeyRef env var and the SCRAM credentials as a mounted
// jaas.conf, both described by the ACLConfig chart values.
func buildSecurityConfigs(sp *kafkamod.SecurityProfile, sec KafkaSecurity) map[string]string {
	if !hasSecurityProfile(sp) {
		return nil
	}
	sec = sec.withDefaults()

	cfg := map[string]string{}
	cfg[keyConsumerSecurityProtocol] = sp.SecurityProtocol
	if sp.SaslMechanism != "" {
		cfg[keyConsumerSaslMechanism] = sp.SaslMechanism
	}

	if usesSSLMaterial(sp) {
		if sp.SSLProtocol != "" {
			cfg[keyConsumerSSLProtocol] = sp.SSLProtocol
		}
		if sp.SSLTruststoreType != "" {
			cfg[keyConsumerSSLTruststoreType] = sp.SSLTruststoreType
		}
		if sp.SSLCertSecret != "" {
			fileName := truststoreFileName(sp.SSLTruststoreType)
			cfg[keyConsumerSSLTruststoreLocation] = certMountPath + "/" + fileName
			// the chart selects this key out of the cert secret and projects it
			// under the same name.
			cfg[keyConsumerSSLTruststoreFilename] = fileName
		}
	}

	// OAUTHBEARER authenticates with the projected service-account token, so the
	// JAAS config is a fixed module string rather than credentials.
	if sp.SaslMechanism == saslMechanismOauthbearer {
		cfg[keyConsumerSaslJaasConfig] = oauthConsumerSaslJaasConfig
		cfg[keyConsumerSaslLoginCallbackHandler] = sec.SaslLoginCallbackHandlerClass
	}

	return cfg
}

// ACLConfig is the chart-facing description of a stream's security material.
// Every field is a reference to a secret that already exists in the target
// namespace — no secret value passes through entropy.
type ACLConfig struct {
	// SSLConfigCredential is the secret holding the truststore. The chart mounts
	// it at /etc/secret, selecting TruststoreFilename as both key and path.
	SSLConfigCredential string `json:"ssl_config_credential,omitempty"`
	TruststoreFilename  string `json:"truststore_filename,omitempty"`

	// TruststorePassword is rendered by the chart as a secretKeyRef env var for
	// SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD.
	TruststorePassword *SecretKeyRef `json:"truststore_password,omitempty"`

	// JaasConfigCredential is the secret holding jaas.conf for PLAIN/SCRAM
	// streams, mounted at /etc/secret/kafka.
	JaasConfigCredential string `json:"jaas_config_credential,omitempty"`

	// KafkaTokenEnabled requests the projected kafka service-account token
	// (audience "kafka") that OAUTHBEARER authenticates with.
	KafkaTokenEnabled bool `json:"kafka_token_enabled,omitempty"`
}

// SecretKeyRef references a single key inside an existing secret.
type SecretKeyRef struct {
	SecretName string `json:"secretName"`
	Key        string `json:"key"`
}

// buildACLConfig derives the chart values for the stream's security material.
// Returns nil when the stream needs none, so the rendered release is unchanged
// for plaintext firehoses.
func buildACLConfig(streamName string, sp *kafkamod.SecurityProfile, team string) *ACLConfig {
	if !hasSecurityProfile(sp) {
		return nil
	}

	acl := &ACLConfig{}

	if usesSSLMaterial(sp) && sp.SSLCertSecret != "" {
		acl.SSLConfigCredential = sp.SSLCertSecret
		acl.TruststoreFilename = truststoreFileName(sp.SSLTruststoreType)
		if sp.SSLTruststorePasswordDetails != nil && sp.SSLTruststorePasswordDetails.SecretName != "" {
			acl.TruststorePassword = &SecretKeyRef{
				SecretName: sp.SSLTruststorePasswordDetails.SecretName,
				Key:        sp.SSLTruststorePasswordDetails.Key,
			}
		}
	}

	if sp.SaslMechanism == saslMechanismOauthbearer {
		acl.KafkaTokenEnabled = true
	}

	// PLAIN/SCRAM read their credentials from a jaas.conf in a secret. The
	// profile names it explicitly when known; otherwise fall back to odin's
	// <team>-<stream>-jaas convention.
	if isPlainOrScramStream(sp) {
		if secretName := jaasSecretName(streamName, sp, team); secretName != "" {
			acl.JaasConfigCredential = secretName
		}
	}

	if *acl == (ACLConfig{}) {
		return nil
	}
	return acl
}

func jaasSecretName(streamName string, sp *kafkamod.SecurityProfile, team string) string {
	if cred, ok := sp.ACLs[team]; ok && cred.SecretName != "" {
		return cred.SecretName
	}
	if team == "" || streamName == "" {
		return ""
	}
	return strings.ReplaceAll(strings.Join([]string{team, streamName, jaasSecretSuffix}, "-"), "_", "-")
}

// applyStreamSecurity resolves the source stream's kafka security profile,
// injects the consumer config into the env variables, and records the chart's
// ACL values on conf. It is a no-op for firehoses that do not name a kafka
// stream, and clears the wiring for streams that no longer carry a profile.
func (fd *firehoseDriver) applyStreamSecurity(ctx context.Context, exr module.ExpandedResource, conf *Config) error {
	// the flag may arrive as an env variable (Dex); it is transient and must
	// never reach the running firehose.
	if val, ok := conf.EnvVariables[keySourceKafkaSecurityEnabled]; ok {
		enabled, err := strconv.ParseBool(strings.TrimSpace(val))
		if err != nil {
			return fmt.Errorf("invalid %s value %q: %w", keySourceKafkaSecurityEnabled, val, err)
		}
		conf.StreamSecurityEnabled = conf.StreamSecurityEnabled || enabled
		delete(conf.EnvVariables, keySourceKafkaSecurityEnabled)
	}

	streamName := conf.streamName()
	if streamName == "" {
		return nil
	}

	if conf.EnvVariables == nil {
		conf.EnvVariables = map[string]string{}
	}
	clearManagedSecurityConfigs(conf.EnvVariables)
	conf.ACL = nil

	security, err := fd.resolveStreamSecurity(ctx, exr, conf, streamName)
	if err != nil {
		return err
	}
	if !hasSecurityProfile(security) {
		return nil
	}

	for key, val := range buildSecurityConfigs(security, fd.conf.KafkaSecurity) {
		conf.EnvVariables[key] = val
	}
	conf.ACL = buildACLConfig(streamName, security, conf.Team)

	// PLAIN/SCRAM point the JVM at the mounted jaas.conf.
	if conf.ACL != nil && conf.ACL.JaasConfigCredential != "" {
		conf.EnvVariables[keyJavaOptions] = withJaasJavaOption(conf.EnvVariables[keyJavaOptions])
	}

	if conf.ServiceAccount == "" {
		conf.ServiceAccount = fd.conf.KafkaSecurity.ServiceAccount
	}

	return nil
}

// clearManagedSecurityConfigs drops everything a previous plan injected: this
// module owns the SASL/SSL keys for a stream-backed firehose.
func clearManagedSecurityConfigs(env map[string]string) {
	for _, key := range managedSecurityKeys {
		delete(env, key)
	}
	// provider class keys are named after the provider, so sweep by shape.
	for key := range env {
		if strings.HasPrefix(key, keyConsumerConfigProviderClassPrefix) &&
			strings.HasSuffix(key, keyConsumerConfigProviderClassSuffix) {
			delete(env, key)
		}
	}
	if opts := withoutJaasJavaOption(env[keyJavaOptions]); opts != "" {
		env[keyJavaOptions] = opts
	} else if _, ok := env[keyJavaOptions]; ok {
		env[keyJavaOptions] = ""
	}
}

// withJaasJavaOption appends the JAAS location option, keeping the rest of the
// user's _JAVA_OPTIONS and never duplicating the option.
func withJaasJavaOption(opts string) string {
	opts = withoutJaasJavaOption(opts)
	if opts == "" {
		return jaasConfigJavaOpt
	}
	return opts + " " + jaasConfigJavaOpt
}

func withoutJaasJavaOption(opts string) string {
	if !strings.Contains(opts, jaasConfigJavaOpt) {
		return opts
	}
	fields := strings.Fields(opts)
	kept := fields[:0]
	for _, f := range fields {
		if f != jaasConfigJavaOpt {
			kept = append(kept, f)
		}
	}
	return strings.Join(kept, " ")
}

// resolveStreamSecurity resolves the source stream's kafka security profile.
//
// Resolution order: an inline conf.StreamSecurity entry, then a declared kafka
// dependency (raw-Entropy path), then — when the resource carries the
// stream_security_enabled flag (the Dex product path) — the kafka resource
// fetched internally by URN via fd.getResource, with no dependency. Firehoses
// with none of these (plaintext) are left untouched.
func (fd *firehoseDriver) resolveStreamSecurity(ctx context.Context, exr module.ExpandedResource,
	conf *Config, streamName string,
) (*kafkamod.SecurityProfile, error) {
	// 1. inline profile, if Dex prefetched one.
	if security := conf.StreamSecurity[streamName]; security != nil {
		return security, nil
	}

	// 2. declared kafka dependency (raw-Entropy path): also carries the URL.
	if dep, ok := exr.Dependencies[streamName]; ok && dep.Kind == kafkamod.Module.Kind {
		var out kafkamod.Output
		if err := json.Unmarshal(dep.Output, &out); err != nil {
			return nil, fmt.Errorf("invalid kafka dependency output for stream %q: %w", streamName, err)
		}
		setKafkaBrokers(conf, out.URL)
		return out.Security, nil
	}

	// 3. flag set (Dex product path): fetch the kafka resource internally by URN
	// and read its security profile — no dependency declared.
	if conf.StreamSecurityEnabled {
		out, err := fd.fetchKafkaOutput(ctx, exr.Resource.Project, streamName)
		if err != nil {
			return nil, err
		}
		setKafkaBrokers(conf, out.URL)
		return out.Security, nil
	}

	return nil, nil
}

// fetchKafkaOutput fetches the kafka stream's resource by URN and decodes its
// Output (url + security profile). streamName is the kafka resource name.
func (fd *firehoseDriver) fetchKafkaOutput(ctx context.Context, project, streamName string) (kafkamod.Output, error) {
	var out kafkamod.Output
	if fd.getResource == nil {
		return out, fmt.Errorf("cannot resolve kafka stream %q: resource getter not configured", streamName)
	}

	urn := resource.GenerateURN(kafkamod.Module.Kind, project, streamName)
	res, err := fd.getResource(ctx, urn)
	if err != nil {
		return out, fmt.Errorf("failed to fetch kafka stream %q (%s): %w", streamName, urn, err)
	}
	if err := json.Unmarshal(res.State.Output, &out); err != nil {
		return out, fmt.Errorf("invalid kafka output for stream %q: %w", streamName, err)
	}
	return out, nil
}

// setKafkaBrokers fills SOURCE_KAFKA_BROKERS from the resolved stream URL,
// leaving an explicitly configured value untouched.
func setKafkaBrokers(conf *Config, url string) {
	if url == "" || conf.EnvVariables[confKeyKafkaBrokers] != "" {
		return
	}
	conf.EnvVariables[confKeyKafkaBrokers] = url
}
