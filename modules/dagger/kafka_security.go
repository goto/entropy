package dagger

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/goto/entropy/core/module"
	kafkamod "github.com/goto/entropy/modules/kafka"
)

// SASL/SSL consumer config keys injected into
// SOURCE_KAFKA_CONSUMER_ADDITIONAL_CONFIGURATIONS.
const (
	keyConsumerSecurityProtocol             = "SOURCE_KAFKA_CONSUMER_CONFIG_SECURITY_PROTOCOL"
	keyConsumerSaslMechanism                = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_MECHANISM"
	keyConsumerSaslJaasConfig               = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_JAAS_CONFIG"
	keyConsumerSaslLoginCallbackHandler     = "SOURCE_KAFKA_CONSUMER_CONFIG_SASL_LOGIN_CALLBACK_HANDLER_CLASS"
	keyConsumerSSLProtocol                  = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_PROTOCOL"
	keyConsumerSSLTruststoreType            = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_TYPE"
	keyConsumerSSLTruststoreLocation        = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_LOCATION"
	keyConsumerSSLTruststorePassword        = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD"
	keyConsumerSSLTruststorePasswordDetails = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_TRUSTSTORE_PASSWORD_DETAILS"
	keyConsumerSSLCertSecret                = "SOURCE_KAFKA_CONSUMER_CONFIG_SSL_CERT_SECRET"
	keyConsumerConfigProviders              = "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS"
	keyConsumerConfigProvidersLiteralClass  = "SOURCE_KAFKA_CONSUMER_CONFIG_CONFIG_PROVIDERS_LITERALFILE_CLASS"
)

// GTF Kafka security constants (mirrors odin app/dagger/constants.js).
const (
	oauthSaslLoginCallbackHandlerClass = "io.gtflabs.kafka.security.oauthbearer.kubernetes.PodLoginCallbackHandler"
	oauthConsumerSaslJaasConfig        = "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required;"
	scramLoginModule                   = "org.apache.kafka.common.security.scram.ScramLoginModule"
	plainLoginModule                   = "org.apache.kafka.common.security.plain.PlainLoginModule"
	literalFileConfigProviderClass     = "com.gtf.dagger.kafka.configproviders.LiteralFileConfigProvider"
	literalFileConfigProviderName      = "literalfile"
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
// (R3) and the podTemplate mounts (R4).
const (
	kafkaTokenVolumeName    = "kafka-token"
	kafkaTokenMountPath     = "/var/run/secrets/kafka/serviceaccount"
	certsMountPathFmt       = "/var/secrets/%s/certs"
	passwordsMountPathFmt   = "/var/secrets/%s/passwords"
	credentialsMountPathFmt = "/var/secrets/%s/credentials"
)

var invalidVolumeNameChars = regexp.MustCompile(`[^a-zA-Z0-9]+`)

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

// buildAdditionalConfigurations builds the SOURCE_KAFKA_CONSUMER_ADDITIONAL_CONFIGURATIONS
// map for a source stream, branching on its security profile. streamName is the
// stable per-stream directory name used both here and in the podTemplate mounts.
// Returns nil for plaintext streams so STREAMS stays unchanged.
func buildAdditionalConfigurations(streamName string, sp *kafkamod.SecurityProfile, team string) map[string]interface{} {
	if !hasSecurityProfile(sp) {
		return nil
	}

	cfg := map[string]interface{}{}
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
			cfg[keyConsumerSSLCertSecret] = sp.SSLCertSecret
		}
		if sp.SSLTruststorePasswordDetails != nil {
			// map (not struct) so JSON key order is deterministic and matches
			// the reference fixture ({key, secretName}).
			cfg[keyConsumerSSLTruststorePasswordDetails] = map[string]string{
				"key":        sp.SSLTruststorePasswordDetails.Key,
				"secretName": sp.SSLTruststorePasswordDetails.SecretName,
			}
		}
	}

	if isOauthbearerStream(sp) {
		cfg[keyConsumerSaslLoginCallbackHandler] = oauthSaslLoginCallbackHandlerClass
		cfg[keyConsumerSaslJaasConfig] = oauthConsumerSaslJaasConfig
		cfg[keyConsumerSSLTruststoreLocation] = fmt.Sprintf(
			"/var/secrets/%s/certs/truststore.%s", streamName, truststoreExtension(sp.SSLTruststoreType))
		if sp.SSLTruststorePasswordDetails != nil {
			cfg[keyConsumerSSLTruststorePassword] = fmt.Sprintf(
				"${literalfile:/var/secrets/%s/passwords/%s:literal-value}",
				streamName, sp.SSLTruststorePasswordDetails.Key)
		}
		cfg[keyConsumerConfigProviders] = literalFileConfigProviderName
		cfg[keyConsumerConfigProvidersLiteralClass] = literalFileConfigProviderClass
	}

	if isPlainOrScramStream(sp) {
		cfg[keyConsumerSaslJaasConfig] = buildSASLJaasConfig(streamName, sp, team)
		// credentials are referenced via the literalfile provider (never inlined).
		if _, ok := sp.ACLs[team]; ok {
			cfg[keyConsumerConfigProviders] = literalFileConfigProviderName
			cfg[keyConsumerConfigProvidersLiteralClass] = literalFileConfigProviderClass
		}
	}

	return cfg
}

// buildSASLJaasConfig builds the JAAS config string for PLAIN/SCRAM mechanisms.
// Credentials are referenced through the literalfile config provider pointing
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

// buildACLMounts derives the podTemplate volume mounts for the given sources'
// security profiles. Returns nil when no source needs ACL mounts so the
// podTemplate is unchanged for plaintext daggers.
func buildACLMounts(sources []Source, profiles map[string]*kafkamod.SecurityProfile, team string) []ACLMount {
	var mounts []ACLMount

	for _, src := range sources {
		streamName := src.SourceKafkaName
		sp := profiles[streamName]
		if !hasSecurityProfile(sp) {
			continue
		}

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

// applyStreamSecurity resolves each source's kafka security profile, injects the
// SASL/SSL consumer config into STREAMS, and records the podTemplate ACL mounts
// on conf. It is a no-op (leaves conf untouched) for daggers whose sources have
// no security profile.
func applyStreamSecurity(exr module.ExpandedResource, conf *Config) error {
	profiles, err := resolveSourceStreams(exr, conf)
	if err != nil { 
		return err
	}
	conf.ACLMounts = buildACLMounts(conf.Source, profiles, conf.Team)
	return nil
}

// resolveSourceStreams resolves each source's kafka security profile (keyed by
// SOURCE_KAFKA_NAME), populates bootstrap servers and the SASL/SSL consumer
// config, and returns the resolved security profiles keyed by stream name.
//
// The profile is resolved inline-first: conf.StreamSecurity (populated by Dex on
// the product path) takes precedence; otherwise it falls back to the kafka
// dependency Output (raw-Entropy path). Sources with neither a profile nor a
// plaintext profile are left untouched — preserving byte-for-byte identical
// STREAMS for ODS daggers.
func resolveSourceStreams(exr module.ExpandedResource, conf *Config) (map[string]*kafkamod.SecurityProfile, error) {
	profiles := map[string]*kafkamod.SecurityProfile{}

	for i := range conf.Source {
		streamName := conf.Source[i].SourceKafkaName
		if streamName == "" {
			continue
		}

		// inline profile (Dex product path) wins over the kafka dependency.
		security := conf.StreamSecurity[streamName]

		// fall back to the kafka dependency (raw-Entropy path): it also carries
		// the resolved broker URL for bootstrap servers.
		if security == nil {
			if dep, ok := exr.Dependencies[streamName]; ok && dep.Kind == kafkamod.Module.Kind {
				var out kafkamod.Output
				if err := json.Unmarshal(dep.Output, &out); err != nil {
					return nil, fmt.Errorf("invalid kafka dependency output for stream %q: %w", streamName, err)
				}

				// bootstrap servers: explicit source value wins, else resolved URL.
				if conf.Source[i].SourceKafkaConsumerConfigBootstrapServers == "" && out.URL != "" {
					conf.Source[i].SourceKafkaConsumerConfigBootstrapServers = out.URL
				}

				security = out.Security
			}
		}

		if !hasSecurityProfile(security) {
			continue
		}

		profiles[streamName] = security
		conf.Source[i].SourceKafkaConsumerAdditionalConfigurations =
			buildAdditionalConfigurations(streamName, security, conf.Team)
	}

	return profiles, nil
}
