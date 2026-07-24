package masking

import (
	"context"
	"encoding/json"
	"fmt"
)

// ModuleConfigLookup resolves the raw configs JSON of a module by its URN. It is
// satisfied by an adapter over core/module.Service.GetModule, keeping this
// package free of the module domain type.
type ModuleConfigLookup interface {
	// ModuleConfigs returns the module's raw configs. It must return an error
	// that unwraps to a not-found condition when the module does not exist so
	// the caller can fail open.
	ModuleConfigs(ctx context.Context, moduleURN string) (json.RawMessage, error)
}

// Provider resolves the sensitive_config path list for a resource's
// (kind, project). It caches lookups per module URN for the lifetime of the
// instance, so a Provider should be created once per request and reused across
// the resources mapped in that request (e.g. within a single ListResources).
//
// A Provider is not safe for concurrent use; mapping within a request is
// sequential.
type Provider struct {
	lookup ModuleConfigLookup
	cache  map[string][]string
}

// NewProvider builds a request-scoped Provider backed by lookup.
func NewProvider(lookup ModuleConfigLookup) *Provider {
	return &Provider{
		lookup: lookup,
		cache:  map[string][]string{},
	}
}

// PathsFor returns the sensitive_config paths for the module owning
// (kind, project). Results are cached per module URN. A missing module returns
// the lookup's error (unwrapping to not-found), which callers treat as
// fail-open.
func (p *Provider) PathsFor(ctx context.Context, kind, project string) ([]string, error) {
	urn := moduleURN(kind, project)
	if paths, ok := p.cache[urn]; ok {
		return paths, nil
	}

	configs, err := p.lookup.ModuleConfigs(ctx, urn)
	if err != nil {
		return nil, err
	}

	paths, err := PathsFromConfigs(configs)
	if err != nil {
		return nil, err
	}
	p.cache[urn] = paths
	return paths, nil
}

// PathsFromConfigs parses the sensitive_config list out of a raw module configs
// payload. A missing key yields nil with no error.
func PathsFromConfigs(configs json.RawMessage) ([]string, error) {
	if len(configs) == 0 {
		return nil, nil
	}
	var env struct {
		SensitiveConfig []string `json:"sensitive_config"`
	}
	if err := json.Unmarshal(configs, &env); err != nil {
		return nil, fmt.Errorf("masking: parse module configs: %w", err)
	}
	return env.SensitiveConfig, nil
}

// moduleURN builds the module URN for a (kind, project). Module URNs order
// project before kind: orn:entropy:module:{project}:{kind}.
func moduleURN(kind, project string) string {
	return fmt.Sprintf("orn:entropy:module:%s:%s", project, kind)
}
