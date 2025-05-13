package kube

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/container/v1"
	"k8s.io/client-go/rest"

	"github.com/goto/entropy/pkg/errors"
)

const (
	providerTypeGKE = "gke"
)

type HelmConfig struct {
	MaxHistory int `json:"max_history" default:"256"`
}

type Config struct {
	// Host - The hostname (in form of URI) of Kubernetes master.
	Host string `json:"host"`

	Timeout time.Duration `json:"timeout" default:"100ms"`

	// Token - Token to authenticate with static oauth2 access token
	Token string `json:"token"`

	// Insecure - Whether server should be accessed without verifying the TLS certificate.
	Insecure bool `json:"insecure" default:"false"`

	// ClientKey - PEM-encoded client key for TLS authentication.
	ClientKey string `json:"client_key"`

	// ClientCertificate - PEM-encoded client certificate for TLS authentication.
	ClientCertificate string `json:"client_certificate"`

	// ClusterCACertificate - PEM-encoded root certificates bundle for TLS authentication.
	ClusterCACertificate string `json:"cluster_ca_certificate"`

	// ProviderType indicates which provider that hos k8s: gke, eks, etc...
	// If it is `gke`, entropy will fetch auth from the default source
	// left it empty if token or client key will be used
	ProviderType string `json:"provider_type"`

	// Namespace defines where the resources that depend on this kube resource deployed to
	// namespace is optional, if it is being defined, it will force all resources that depend
	// on this kube resource to be deployed to the defined namespace
	Namespace string `json:"namespace"`

	HelmConfig HelmConfig `json:"helm_config"`
}

func (conf *Config) RESTConfig(ctx context.Context) (*rest.Config, error) {
	restTLSClientConfig := rest.TLSClientConfig{
		Insecure: conf.Insecure,
	}
	if conf.ClusterCACertificate != "" {
		restTLSClientConfig.CAData = []byte(conf.ClusterCACertificate)
	}
	if conf.ClientKey != "" {
		restTLSClientConfig.KeyData = []byte(conf.ClientKey)
	}
	if conf.ClientCertificate != "" {
		restTLSClientConfig.CertData = []byte(conf.ClientCertificate)
	}

	rc := &rest.Config{
		Host:            conf.Host,
		Timeout:         conf.Timeout,
		TLSClientConfig: restTLSClientConfig,
	}

	// Override with req's Transport.
	tlsConfig, err := rest.TLSConfigFor(rc)
	if err != nil {
		return nil, errors.ErrInternal.WithMsgf("kube: failed to create TLS config").WithCausef(err.Error())
	}

	defaultTransport := http.DefaultTransport.(*http.Transport).Clone()
	defaultTransport.TLSClientConfig = tlsConfig
	transport := otelhttp.NewTransport(defaultTransport)
	rc.Transport = transport
	// rest.Config.TLSClientConfig should be empty if
	// custom Transport been set.
	rc.TLSClientConfig = rest.TLSClientConfig{}

	if conf.ProviderType != "" {
		switch conf.ProviderType {
		case providerTypeGKE:
			ts, err := google.DefaultTokenSource(ctx, container.CloudPlatformScope)
			if err != nil {
				return nil, errors.ErrInvalid.WithMsgf("%s: can't fetch credentials from service account json", conf.ProviderType).WithCausef(err.Error())
			}
			oauth2Token, err := ts.Token()
			if err != nil {
				return nil, errors.ErrInternal.WithMsgf("%s: can't get token from token source", conf.ProviderType).WithCausef(err.Error())
			}
			rc.BearerToken = oauth2Token.AccessToken
			conf.Token = oauth2Token.AccessToken
		default:
			return nil, errors.ErrInternal.WithMsgf("%s: unsupported provider type", conf.ProviderType)
		}
	} else if conf.Token != "" {
		rc.BearerToken = conf.Token
	}

	return rc, nil
}

func (conf *Config) StreamingConfig(ctx context.Context) (*rest.Config, error) {
	rc, err := conf.RESTConfig(ctx)
	if err != nil {
		return nil, err
	}
	rc.Timeout = 0
	return rc, nil
}

func (conf *Config) Sanitise() error {
	if conf.Host == "" {
		return errors.ErrInvalid.WithMsgf("host must be set")
	}

	if conf.Timeout == 0 {
		conf.Timeout = 1 * time.Second
	}

	if conf.ProviderType == "" {
		if conf.Token == "" {
			if conf.ClientKey == "" || conf.ClientCertificate == "" {
				return errors.ErrInvalid.
					WithMsgf("client_key and client_certificate must be set when token and service account is not set")
			}
		}

		if !conf.Insecure && len(conf.ClusterCACertificate) == 0 {
			return errors.ErrInvalid.WithMsgf("cluster_ca_certificate must be set when insecure=false")
		}
	}

	return nil
}
