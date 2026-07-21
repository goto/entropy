package kafka

import (
	_ "embed"
	"encoding/json"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

var (
	//go:embed schema/config.json
	configSchemaRaw []byte
	validateConfig  = validator.FromJSONSchema(configSchemaRaw)
)

type Config struct {
	Entity        string           `json:"entity,omitempty"`
	Environment   string           `json:"environment,omitempty"`
	Landscape     string           `json:"landscape,omitempty"`
	Organization  string           `json:"organization,omitempty"`
	AdvertiseMode AdvertiseMode    `json:"advertise_mode"`
	Brokers       []Broker         `json:"brokers,omitempty"`
	Type          string           `json:"type"`
	Security      *SecurityProfile `json:"security,omitempty"`
}

// SecurityProfile carries the optional SASL/SSL authentication details for a
// stream. When nil (or empty SecurityProtocol), the stream is treated as a
// plaintext stream and behaves exactly as before. Only references to secrets
// are stored here — never inline secret values.
type SecurityProfile struct {
	// SecurityProtocol is the Kafka security.protocol, e.g. SASL_SSL,
	// SASL_PLAINTEXT, SSL or empty/PLAINTEXT.
	SecurityProtocol string `json:"security_protocol,omitempty"`
	// SaslMechanism is the SASL mechanism, e.g. OAUTHBEARER, PLAIN,
	// SCRAM-SHA-512.
	SaslMechanism string `json:"sasl_mechanism,omitempty"`
	// SSLProtocol is the ssl.protocol, e.g. SSL / TLS.
	SSLProtocol string `json:"ssl_protocol,omitempty"`
	// SSLTruststoreType is the truststore type, e.g. PKCS12 or JKS.
	SSLTruststoreType string `json:"ssl_truststore_type,omitempty"`
	// SSLCertSecret is the name of the K8s secret holding the truststore/certs.
	SSLCertSecret string `json:"ssl_cert_secret,omitempty"`
	// SSLTruststorePasswordDetails references the secret + key holding the
	// truststore password.
	SSLTruststorePasswordDetails *SecretKeyRef `json:"ssl_truststore_password_details,omitempty"`
	// ACLs holds per-team credential references for PLAIN/SCRAM mechanisms,
	// keyed by team/group. References only — never inline secret values.
	ACLs map[string]ACLCredentialRef `json:"acls,omitempty"`
}

// SecretKeyRef is a reference to a single key inside a K8s secret.
type SecretKeyRef struct {
	SecretName string `json:"secretName,omitempty"`
	Key        string `json:"key,omitempty"`
}

// ACLCredentialRef references the secret material for a PLAIN/SCRAM credential.
// It never carries inline username/password values.
type ACLCredentialRef struct {
	// SecretName is the K8s secret holding the credential material.
	SecretName string `json:"secretName,omitempty"`
	// UsernameKey / PasswordKey are the keys inside SecretName.
	UsernameKey string `json:"usernameKey,omitempty"`
	PasswordKey string `json:"passwordKey,omitempty"`
}

type AdvertiseMode struct {
	Host    string `json:"host"`
	Address string `json:"address"`
}

type Broker struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Address string `json:"address"`
}

func readConfig(res resource.Resource, confJSON json.RawMessage, dc driverConf) (*Config, error) {
	cfg := Config{
		Type:         dc.Type,
		Entity:       dc.Entity,
		Organization: dc.Organization,
		Landscape:    dc.Landscape,
		Environment:  dc.Environment,
	}

	if res.Spec.Configs != nil {
		if err := json.Unmarshal(res.Spec.Configs, &cfg); err != nil {
			return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal").WithCausef("%s", err.Error())
		}
	}

	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal").WithCausef("%s", err.Error())
	}

	newConfJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.ErrInvalid.WithMsgf("failed to marshal").WithCausef("%s", err.Error())
	}

	if err := validateConfig(newConfJSON); err != nil {
		return nil, err
	}

	return &cfg, nil
}
