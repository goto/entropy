package helm

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	memcached "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube"
)

// Config contains Helm CLI configuration parameters.
type Config struct {
	HelmDriver string `default:"secret"` // values: configmap/secret/memory/sql
	Kubernetes kube.Config
}

// kubeClientGetter is a RESTClientGetter interface implementation
// comes from https://github.com/hashicorp/terraform-provider-helm
type kubeClientGetter struct {
	ClientConfig clientcmd.ClientConfig
}

func (k *kubeClientGetter) ToRESTConfig() (*rest.Config, error) {
	config, err := k.ToRawKubeConfigLoader().ClientConfig()

	// Override with req's Transport.
	tlsConfig, err := rest.TLSConfigFor(config)
	if err != nil {
		return nil, errors.ErrInternal.WithMsgf("helm: failed to create TLS config").WithCausef(err.Error())
	}

	defaultTransport := http.DefaultTransport.(*http.Transport).Clone()
	defaultTransport.TLSClientConfig = tlsConfig
	transport := otelhttp.NewTransport(defaultTransport)
	config.Transport = transport
	// rest.Config.TLSClientConfig should be empty if
	// custom Transport been set.
	config.TLSClientConfig = rest.TLSClientConfig{}

	return config, err
}

func (k *kubeClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := k.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with one-ish version
	// each, discovery needs to make 50 requests double it just so we don't end
	// up here again for a while.  This config is only used for discovery.
	config.Burst = 100

	return memcached.NewMemCacheClient(discovery.NewDiscoveryClientForConfigOrDie(config)), nil
}

func (k *kubeClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := k.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (k *kubeClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return k.ClientConfig
}
