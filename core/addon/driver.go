package addon

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/goto/entropy/pkg/errors"
)

// Driver implements a single add-on type.
type Driver interface {
	// Validate checks the shape of an add-on spec.
	Validate(spec json.RawMessage) error

	// RequiresParentCompleted reports whether Apply needs the parent resource
	// in STATUS_COMPLETED (e.g., to read its output).
	RequiresParentCompleted() bool

	// RequiresConfig reports whether the type needs a project addon-config.
	// When false, config passed to Apply/Delete is nil.
	RequiresConfig() bool

	// SensitiveConfigKeys lists top-level addon-config keys masked on read.
	SensitiveConfigKeys() []string

	// Apply creates or updates the add-on and returns its output. It must be
	// idempotent for the same parent and spec.
	Apply(ctx context.Context, parent ParentContext, config, spec json.RawMessage) (json.RawMessage, error)

	// Delete removes whatever Apply created. It must be idempotent.
	Delete(ctx context.Context, parent ParentContext, config, spec json.RawMessage) error
}

// ParentContext is the read-only view of the parent resource given to drivers.
type ParentContext struct {
	URN     string
	Kind    string
	Project string
	Name    string
	Output  json.RawMessage
}

// Registry holds the drivers for all supported add-on types.
type Registry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

// Register adds the driver for the given add-on type.
func (r *Registry) Register(typ string, drv Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.drivers == nil {
		r.drivers = map[string]Driver{}
	}
	if _, exists := r.drivers[typ]; exists {
		return errors.ErrConflict.WithMsgf("add-on type '%s' is already registered", typ)
	}
	r.drivers[typ] = drv
	return nil
}

// Get returns the driver for the given add-on type.
func (r *Registry) Get(typ string) (Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	drv, found := r.drivers[typ]
	if !found {
		return nil, errors.ErrInvalid.WithMsgf("add-on type '%s' is not supported", typ)
	}
	return drv, nil
}
