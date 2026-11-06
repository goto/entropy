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

// Meta is implemented by meta add-ons. A meta add-on keeps no addons row: it is
// enabled on every parent whose kind supports it unless the project addon-config
// sets "enabled": false, and is computed on read.
type Meta interface {
	// Output returns the computed output of the add-on on the parent.
	Output(parent ParentContext) json.RawMessage
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
	metas   map[string]Meta
}

// Register adds the driver for the given add-on type.
func (r *Registry) Register(typ string, drv Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.registered(typ) {
		return errors.ErrConflict.WithMsgf("add-on type '%s' is already registered", typ)
	}
	if r.drivers == nil {
		r.drivers = map[string]Driver{}
	}
	r.drivers[typ] = drv
	return nil
}

// RegisterMeta adds the meta add-on of the given type.
func (r *Registry) RegisterMeta(typ string, m Meta) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.registered(typ) {
		return errors.ErrConflict.WithMsgf("add-on type '%s' is already registered", typ)
	}
	if r.metas == nil {
		r.metas = map[string]Meta{}
	}
	r.metas[typ] = m
	return nil
}

// Get returns the driver for the given add-on type.
func (r *Registry) Get(typ string) (Driver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, isMeta := r.metas[typ]; isMeta {
		return nil, errors.ErrInvalid.WithMsgf("add-on '%s' is a meta add-on and cannot be written; disable it with the project addon-config", typ)
	}
	drv, found := r.drivers[typ]
	if !found {
		return nil, errors.ErrInvalid.WithMsgf("add-on type '%s' is not supported", typ)
	}
	return drv, nil
}

// GetMeta returns the meta add-on of the given type, if any.
func (r *Registry) GetMeta(typ string) (Meta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	m, found := r.metas[typ]
	return m, found
}

// sensitiveKeys returns the addon-config keys to mask for the given type.
func (r *Registry) sensitiveKeys(typ string) ([]string, error) {
	if _, isMeta := r.GetMeta(typ); isMeta {
		return nil, nil
	}
	drv, err := r.Get(typ)
	if err != nil {
		return nil, err
	}
	return drv.SensitiveConfigKeys(), nil
}

func (r *Registry) registered(typ string) bool {
	_, isDriver := r.drivers[typ]
	_, isMeta := r.metas[typ]
	return isDriver || isMeta
}
