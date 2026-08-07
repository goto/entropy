package firehose

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	kafkamod "github.com/goto/entropy/modules/kafka"
)

// SASL/SSL consumer config keys. Firehose passes every
// SOURCE_KAFKA_CONSUMER_CONFIG_* env variable straight to the kafka consumer,
// so unlike dagger — where these live inside the STREAMS json — they are flat
// env variables here.
const (
	keyConsumerSecurityProtocol            = "SOURCE_KAFKA_CONSUMER_CONFIG_SECURITY_PROTOCOL"
	keyConsumerSaslMechanism               = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_MECHANISM"
	keyConsumerSaslJaasConfig              = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_JAAS_CONFIG"
	keyConsumerSaslLoginCallbackHandler    = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_LOGIN_CALLBACK_HANDLER_CLASS"
	keyConsumerSSLProtocol                 = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_PROTOCOL"
	keyConsumerSSLTruststoreType           = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_TYPE"
	keyConsumerSSLTruststoreLocation       = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_LOCATION"
	keyConsumerSSLTruststorePassword       = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD"
	keyConsumerConfigProviders             = "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS"
	keyConsumerConfigProvidersLiteralClass = "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS_LITERALFILE_CLASS"
)

// transient inputs Dex may send as env variables. They describe the stream to
// resolve and are never forwarded to the running firehose.
const (
	keySourceKafkaName            = "SOURCE_KAFKA_NAME"
	keySourceKafkaSecurityEnabled = "SOURCE_KAFKA_SECURITY_ENABLED"
)

// managedSecurityKeys are owned by this module: they are wiped and rebuilt on
// every plan of a firehose that names a kafka stream, so a stream that loses
// its ACLs does not leave stale credentials behind.
var managedSecurityKeys = []string{
	keyConsumerSecurityProtocol,
	keyConsumerSaslMechanism,
	keyConsumerSaslJaasConfig,
	keyConsumerSaslLoginCallbackHandler,
	keyConsumerSSLProtocol,
	keyConsumerSSLTruststoreType,
	keyConsumerSSLTruststoreLocation,
	keyConsumerSSLTruststorePassword,
	keyConsumerConfigProviders,
	keyConsumerConfigProvidersLiteralClass,
}

// GTF Kafka security constants, same values the dagger module injects.
const (
	oauthConsumerSaslJaasConfig   = "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required;"
	scramLoginModule              = "org.apache.kafka.common.security.scram.ScramLoginModule"
	plainLoginModule              = "org.apache.kafka.common.security.plain.PlainLoginModule"
	literalFileConfigProviderName = "literalfile"

	// defaults for the classes shipped by the platform kafka-security library.
	// Override per deployment via driver config `kafka_security` when the
	// firehose image packages them under different names.
	defaultOauthSaslLoginCallbackHandlerClass = "io.gtflabs.kafka.security.oauthbearer.kubernetes.PodLoginCallbackHandler"
	defaultLiteralFileConfigProviderClass     = "com.gtf.dagger.kafka.configproviders.LiteralFileConfigProvider"
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

// mount path templates, kept consistent between the injected consumer config
// and the pod volume mounts, and identical to the dagger module's layout.
const (
	kafkaTokenVolumeName    = "kafka-token"
	kafkaTokenMountPath     = "/var/run/secrets/kafka/serviceaccount"
	certsMountPathFmt       = "/var/secrets/%s/certs"
	passwordsMountPathFmt   = "/var/secrets/%s/passwords"
	credentialsMountPathFmt = "/var/secrets/%s/credentials"
)

var invalidVolumeNameChars = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// KafkaSecurity carries the deployment level knobs for ACL streams.
type KafkaSecurity struct {
	// ConfigProviderClass reads secret material off the mounted volumes for the
	// ${literalfile:...} references in the injected consumer config.
	ConfigProviderClass string `json:"config_provider_class,omitempty"`

	// SaslLoginCallbackHandlerClass is the OAUTHBEARER login callback handler
	// that exchanges the projected service-account token for a kafka token.
	SaslLoginCallbackHandlerClass string `json:"sasl_login_callback_handler_class,omitempty"`

	// ServiceAccount is the OAuth identity authorized for ACL streams. Used
	// when the resource config does not set one. Empty preserves the chart's
	// default service account.
	ServiceAccount string `json:"service_account,omitempty"`
}

func (k KafkaSecurity) withDefaults() KafkaSecurity {
	if k.ConfigProviderClass == "" {
		k.ConfigProviderClass = defaultLiteralFileConfigProviderClass
	}
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

func isTLSStream(sp *kafkamod.SecurityProfile) bool {
	return sp != nil && sp.SecurityProtocol == securityProtocolSSL
}

// hasSecurityProfile reports whether the profile requires any SASL/SSL wiring.
func hasSecurityProfile(sp *kafkamod.SecurityProfile) bool {
	return sp != nil && sp.SecurityProtocol != "" && !strings.EqualFold(sp.SecurityProtocol, "PLAINTEXT")
}

func truststoreExtension(truststoreType string) string {
	if strings.EqualFold(truststoreType, truststoreTypePKCS12) {
		return "p12"
	}
	return "jks"
}

// buildSecurityConfigs builds the SOURCE_KAFKA_CONSUMER_CONFIG_* env variables
// for the source stream, branching on its security profile. streamName is the
// stable per-stream directory name used both here and in the volume mounts.
// Returns nil for plaintext streams so env variables stay unchanged.
//
// Unlike dagger, the mount descriptors (SSL_CERT_SECRET and
// SSL_TRUSTSTORE_PASSWORD_DETAILS) are not injected: firehose hands these keys
// to the kafka consumer verbatim, so the secret references travel through
// Config.ACLMounts (the chart values) instead.
func buildSecurityConfigs(streamName string, sp *kafkamod.SecurityProfile, team string, sec KafkaSecurity) map[string]string {
	if !hasSecurityProfile(sp) {
		return nil
	}
	sec = sec.withDefaults()

	cfg := map[string]string{}
	cfg[keyConsumerSecurityProtocol] = sp.SecurityProtocol
	if sp.SaslMechanism != "" {
		cfg[keyConsumerSaslMechanism] = sp.SaslMechanism
	}

	// SSL material is shared by the TLS and OAUTHBEARER paths.
	if isTLSStream(sp) || isOauthbearerStream(sp) {
		if sp.SSLProtocol != "" {
			cfg[keyConsumerSSLProtocol] = sp.SSLProtocol
		}
		if sp.SSLTruststoreType != "" {
			cfg[keyConsumerSSLTruststoreType] = sp.SSLTruststoreType
		}
		if sp.SSLCertSecret != "" {
			cfg[keyConsumerSSLTruststoreLocation] = fmt.Sprintf(
				"/var/secrets/%s/certs/truststore.%s", streamName, truststoreExtension(sp.SSLTruststoreType))
		}
		if sp.SSLTruststorePasswordDetails != nil {
			cfg[keyConsumerSSLTruststorePassword] = fmt.Sprintf(
				"${literalfile:/var/secrets/%s/passwords/%s:literal-value}",
				streamName, sp.SSLTruststorePasswordDetails.Key)
			cfg[keyConsumerConfigProviders] = literalFileConfigProviderName
			cfg[keyConsumerConfigProvidersLiteralClass] = sec.ConfigProviderClass
		}
	}

	if isOauthbearerStream(sp) {
		cfg[keyConsumerSaslLoginCallbackHandler] = sec.SaslLoginCallbackHandlerClass
		cfg[keyConsumerSaslJaasConfig] = oauthConsumerSaslJaasConfig
	}

	if isPlainOrScramStream(sp) {
		cfg[keyConsumerSaslJaasConfig] = buildSASLJaasConfig(streamName, sp, team)
		// credentials are referenced via the literalfile provider (never inlined).
		if _, ok := sp.ACLs[team]; ok {
			cfg[keyConsumerConfigProviders] = literalFileConfigProviderName
			cfg[keyConsumerConfigProvidersLiteralClass] = sec.ConfigProviderClass
		}
	}

	return cfg
}

// buildSASLJaasConfig builds the JAAS config string for PLAIN/SCRAM mechanisms.
// Credentials are referenced through the literalfile config provider pointing
// at the mounted secret, never inlined.
func buildSASLJaasConfig(streamName string, sp *kafkamod.SecurityProfile, team string) string {
	loginModule := scramLoginModule
	if sp.SaslMechanism == saslMechanismPlain {
		loginModule = plainLoginModule
	}

	cred, ok := sp.ACLs[team]
	if !ok || cred.SecretName == "" {
		return fmt.Sprintf("%s required;", loginModule)
	}

	userRef := fmt.Sprintf("${literalfile:/var/secrets/%s/credentials/%s:literal-value}", streamName, cred.UsernameKey)
	passRef := fmt.Sprintf("${literalfile:/var/secrets/%s/credentials/%s:literal-value}", streamName, cred.PasswordKey)
	return fmt.Sprintf("%s required username=%q password=%q;", loginModule, userRef, passRef)
}

// sanitizeVolumeName renders a k8s-safe (<=63 char, lowercase alnum/dash) volume name.
func sanitizeVolumeName(name string) string {
	sanitized := invalidVolumeNameChars.ReplaceAllString(name, "-")
	sanitized = strings.ToLower(strings.Trim(sanitized, "-"))
	if len(sanitized) > 63 {
		sanitized = strings.Trim(sanitized[:63], "-")
	}
	return sanitized
}

// buildACLMounts derives the pod volume mounts required by the source stream's
// security profile. Returns nil when the stream needs no ACL mounts so the pod
// spec is unchanged for plaintext firehoses.
func buildACLMounts(streamName string, sp *kafkamod.SecurityProfile, team string) []ACLMount {
	if !hasSecurityProfile(sp) {
		return nil
	}

	var mounts []ACLMount

	if isOauthbearerStream(sp) {
		mounts = append(mounts, ACLMount{
			Name:       sanitizeVolumeName(streamName + "-" + sp.SSLCertSecret),
			MountPath:  fmt.Sprintf(certsMountPathFmt, streamName),
			SecretName: sp.SSLCertSecret,
			Type:       "secret",
		})
		mounts = append(mounts, ACLMount{
			Name:       sanitizeVolumeName(streamName + "-" + sp.SSLTruststorePasswordDetails.SecretName),
			MountPath:  fmt.Sprintf(passwordsMountPathFmt, streamName),
			SecretName: sp.SSLTruststorePasswordDetails.SecretName,
			Type:       "secret",
		})
	} else if isTLSStream(sp) && sp.SSLCertSecret != "" {
		mounts = append(mounts, ACLMount{
			Name:       sanitizeVolumeName(streamName + "-" + sp.SSLCertSecret),
			MountPath:  fmt.Sprintf(certsMountPathFmt, streamName),
			SecretName: sp.SSLCertSecret,
			Type:       "secret",
		})
		if sp.SSLTruststorePasswordDetails != nil && sp.SSLTruststorePasswordDetails.SecretName != "" {
			mounts = append(mounts, ACLMount{
				Name:       sanitizeVolumeName(streamName + "-" + sp.SSLTruststorePasswordDetails.SecretName),
				MountPath:  fmt.Sprintf(passwordsMountPathFmt, streamName),
				SecretName: sp.SSLTruststorePasswordDetails.SecretName,
				Type:       "secret",
			})
		}
	}

	// PLAIN/SCRAM: mount the team's referenced credential secret so the
	// literalfile provider can read username/password without inlining them.
	if isPlainOrScramStream(sp) {
		if cred, ok := sp.ACLs[team]; ok && cred.SecretName != "" {
			mounts = append(mounts, ACLMount{
				Name:       sanitizeVolumeName(streamName + "-" + cred.SecretName),
				MountPath:  fmt.Sprintf(credentialsMountPathFmt, streamName),
				SecretName: cred.SecretName,
				Type:       "secret",
			})
		}
	}

	if len(mounts) == 0 {
		return nil
	}

	// single shared projected kafka service-account token.
	mounts = append(mounts, ACLMount{
		Name:      kafkaTokenVolumeName,
		MountPath: kafkaTokenMountPath,
		Type:      "projected",
	})

	return mounts
}

// applyStreamSecurity resolves the source stream's kafka security profile,
// injects the SASL/SSL consumer config into the env variables, and records the
// pod ACL mounts on conf. It is a no-op (leaves conf untouched) for firehoses
// that do not name a kafka stream, and clears the wiring for streams that no
// longer carry a security profile.
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

	// this module owns the SASL/SSL keys for a stream-backed firehose: drop
	// whatever a previous plan injected before re-resolving.
	for _, key := range managedSecurityKeys {
		delete(conf.EnvVariables, key)
	}
	conf.ACLMounts = nil

	security, err := fd.resolveStreamSecurity(ctx, exr, conf, streamName)
	if err != nil {
		return err
	}
	if !hasSecurityProfile(security) {
		return nil
	}

	for key, val := range buildSecurityConfigs(streamName, security, conf.Team, fd.conf.KafkaSecurity) {
		conf.EnvVariables[key] = val
	}
	conf.ACLMounts = buildACLMounts(streamName, security, conf.Team)

	if conf.ServiceAccount == "" {
		conf.ServiceAccount = fd.conf.KafkaSecurity.ServiceAccount
	}

	return nil
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
