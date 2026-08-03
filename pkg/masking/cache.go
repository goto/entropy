package masking

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
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

// ConfigCache resolves and caches the sensitive_configs path list for a
// resource's (kind, project), for the lifetime of the process. Entries are
// populated lazily via lookup and must be evicted by the caller (module
// Create/Update) whenever a module's configs change, so reads stay correct
// without a TTL.
//
// ConfigCache is safe for concurrent use.
type ConfigCache struct {
	lookup ModuleConfigLookup

	mu    sync.RWMutex
	cache map[string][]string
}

// NewConfigCache builds a ConfigCache backed by lookup.
func NewConfigCache(lookup ModuleConfigLookup) *ConfigCache {
	return &ConfigCache{
		lookup: lookup,
		cache:  map[string][]string{},
	}
}

// PathsFor returns the sensitive_configs paths for the module owning
// (kind, project), populating the cache on a miss. A missing module returns
// the lookup's error (unwrapping to not-found), which callers treat as
// fail-open; the miss is not cached.
func (c *ConfigCache) PathsFor(ctx context.Context, kind, project string) ([]string, error) {
	urn := moduleURN(kind, project)

	c.mu.RLock()
	paths, ok := c.cache[urn]
	c.mu.RUnlock()
	if ok {
		return paths, nil
	}

	configs, err := c.lookup.ModuleConfigs(ctx, urn)
	if err != nil {
		return nil, err
	}

	paths, err = PathsFromConfigs(configs)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[urn] = paths
	c.mu.Unlock()
	return paths, nil
}

// Evict removes the cached sensitive_configs paths for (kind, project), so the
// next PathsFor call re-resolves them from the module's current configs. Call
// this after a module Create/Update.
func (c *ConfigCache) Evict(kind, project string) {
	urn := moduleURN(kind, project)
	c.mu.Lock()
	delete(c.cache, urn)
	c.mu.Unlock()
}

// PathsFromConfigs parses the sensitive_configs list out of a raw module configs
// payload. A missing key yields nil with no error.
func PathsFromConfigs(configs json.RawMessage) ([]string, error) {
	if len(configs) == 0 {
		return nil, nil
	}
	var env struct {
		SensitiveConfigs []string `json:"sensitive_configs"`
	}
	if err := json.Unmarshal(configs, &env); err != nil {
		return nil, fmt.Errorf("masking: parse module configs: %w", err)
	}
	return env.SensitiveConfigs, nil
}

// moduleURN builds the module URN for a (kind, project). Module URNs order
// project before kind: orn:entropy:module:{project}:{kind}.
func moduleURN(kind, project string) string {
	return fmt.Sprintf("orn:entropy:module:%s:%s", project, kind)
}
