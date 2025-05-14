package kube

import (
	"context"
	"time"

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

	// Maximum burst for throttle.
	Burst int `json:"burst" default:"-1"`

	// QPS indicates the maximum QPS to the master from this client.
	QPS float32 `json:"qps" default:"100"`
}

func (conf *Config) RESTConfig(ctx context.Context) (*rest.Config, error) {
	rc := &rest.Config{
		Host:    conf.Host,
		Timeout: conf.Timeout,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: conf.Insecure,
			CAData:   []byte(conf.ClusterCACertificate),
			KeyData:  []byte(conf.ClientKey),
			CertData: []byte(conf.ClientCertificate),
		},
	}

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

	rc.Burst = conf.Burst
	rc.QPS = conf.QPS

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
