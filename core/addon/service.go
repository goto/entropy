package addon

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

// ParentGetter loads live (not soft-deleted) parent resources.
type ParentGetter interface {
	GetByURN(ctx context.Context, urn string) (*resource.Resource, error)
}

// DescriptorGetter returns module descriptors, which declare supported add-ons.
type DescriptorGetter interface {
	GetDescriptor(kind string) (module.Descriptor, error)
}

// ApplyRequest is a request to create or update an add-on.
type ApplyRequest struct {
	ParentURN string
	Type      string
	Spec      json.RawMessage
	// SpecVersion, when set, must match the stored spec_version.
	SpecVersion *int64
	UserID      string
}

// Service manages add-ons. It is the only writer of add-ons, apart from the
// soft-delete cascade of the parent resource in the store.
type Service struct {
	store       Store
	registry    *Registry
	parents     ParentGetter
	descriptors DescriptorGetter
	clock       func() time.Time
	timeout     time.Duration
}

func NewService(store Store, registry *Registry, parents ParentGetter, descriptors DescriptorGetter,
	clock func() time.Time, timeout time.Duration,
) *Service {
	return &Service{
		store:       store,
		registry:    registry,
		parents:     parents,
		descriptors: descriptors,
		clock:       clock,
		timeout:     timeout,
	}
}

// Apply validates the spec, calls the driver and persists the result. On
// failure, an update keeps the previous spec and records ERROR, while a first
// apply inserts the row with ERROR so that it can be retried.
func (svc *Service) Apply(ctx context.Context, req ApplyRequest) (*Addon, error) {
	drv, err := svc.registry.Get(req.Type)
	if err != nil {
		return nil, err
	}

	parent, err := svc.loadParent(ctx, req.ParentURN, req.Type)
	if err != nil {
		return nil, err
	}

	spec := req.Spec
	if len(spec) == 0 {
		spec = json.RawMessage("{}")
	}
	if err := drv.Validate(spec); err != nil {
		return nil, asInvalid(err)
	}

	existing, err := svc.store.GetAddon(ctx, req.ParentURN, req.Type)
	if err != nil && !errors.Is(err, errors.ErrNotFound) {
		return nil, err
	}
	if req.SpecVersion != nil && (existing == nil || existing.SpecVersion != *req.SpecVersion) {
		return nil, errors.ErrConflict.WithMsgf("spec_version %d does not match the stored add-on", *req.SpecVersion)
	}

	if drv.RequiresParentCompleted() && parent.State.Status != resource.StatusCompleted {
		return nil, errors.ErrInvalid.WithMsgf("parent must be in %s before applying add-on '%s'", resource.StatusCompleted, req.Type)
	}

	config, err := svc.loadConfig(ctx, drv, parent.Project, req.Type)
	if err != nil {
		return nil, err
	}

	applyCtx, cancel := context.WithTimeout(ctx, svc.timeout)
	output, applyErr := drv.Apply(applyCtx, parentContext(*parent), config, spec)
	cancel()
	if len(output) == 0 {
		output = json.RawMessage("{}")
	}

	now := svc.clock()
	if existing == nil {
		addon := Addon{
			URN:         GenerateURN(req.ParentURN, req.Type),
			ParentURN:   req.ParentURN,
			Type:        req.Type,
			Spec:        spec,
			Output:      output,
			Status:      StatusCompleted,
			SpecVersion: 1,
			CreatedAt:   now,
			UpdatedAt:   now,
			CreatedBy:   req.UserID,
			UpdatedBy:   req.UserID,
		}
		if applyErr != nil {
			addon.Output = json.RawMessage("{}")
			addon.Status = StatusError
			addon.LastError = applyErr.Error()
		}
		if err := svc.store.CreateAddon(ctx, &addon); err != nil {
			return nil, err
		}
		if applyErr != nil {
			return nil, failed("apply", req.Type, applyErr)
		}
		return &addon, nil
	}

	addon := *existing
	addon.UpdatedAt = now
	addon.UpdatedBy = req.UserID
	if applyErr != nil {
		addon.Status = StatusError
		addon.LastError = applyErr.Error()
	} else {
		addon.Spec = spec
		addon.Output = output
		addon.Status = StatusCompleted
		addon.LastError = ""
		addon.SpecVersion++
	}
	if err := svc.store.UpdateAddon(ctx, addon, existing.SpecVersion); err != nil {
		return nil, err
	}
	if applyErr != nil {
		return nil, failed("apply", req.Type, applyErr)
	}
	return &addon, nil
}

// Get returns the live add-on of the given type on the parent. Meta add-ons
// are computed and returned while enabled.
func (svc *Service) Get(ctx context.Context, parentURN, typ string) (*Addon, error) {
	parent, err := svc.parents.GetByURN(ctx, parentURN)
	if err != nil {
		return nil, err
	}

	if m, isMeta := svc.registry.GetMeta(typ); isMeta {
		a, err := svc.meta(ctx, *parent, typ, m)
		if err != nil {
			return nil, err
		}
		if a == nil {
			return nil, errors.ErrNotFound.WithMsgf("add-on '%s' not found on resource '%s'", typ, parentURN)
		}
		return a, nil
	}
	return svc.store.GetAddon(ctx, parentURN, typ)
}

// List returns the live add-ons of the parent, including enabled meta add-ons.
func (svc *Service) List(ctx context.Context, parentURN string) ([]Addon, error) {
	parent, err := svc.parents.GetByURN(ctx, parentURN)
	if err != nil {
		return nil, err
	}

	addons, err := svc.store.ListAddons(ctx, parentURN)
	if err != nil {
		return nil, err
	}

	desc, err := svc.descriptors.GetDescriptor(parent.Kind)
	if err != nil {
		return nil, err
	}
	for _, typ := range desc.Addons {
		m, isMeta := svc.registry.GetMeta(typ)
		if !isMeta {
			continue
		}
		a, err := svc.meta(ctx, *parent, typ, m)
		if err != nil {
			return nil, err
		}
		if a != nil {
			addons = append(addons, *a)
		}
	}
	return addons, nil
}

// Delete calls the driver to remove external state, then soft-deletes the add-on.
func (svc *Service) Delete(ctx context.Context, parentURN, typ, deletedBy string) error {
	drv, err := svc.registry.Get(typ)
	if err != nil {
		return err
	}

	parent, err := svc.parents.GetByURN(ctx, parentURN)
	if err != nil {
		return err
	}

	existing, err := svc.store.GetAddon(ctx, parentURN, typ)
	if err != nil {
		return err
	}

	config, err := svc.loadConfig(ctx, drv, parent.Project, typ)
	if err != nil {
		return err
	}

	deleteCtx, cancel := context.WithTimeout(ctx, svc.timeout)
	defer cancel()
	if err := drv.Delete(deleteCtx, parentContext(*parent), config, existing.Spec); err != nil {
		return failed("delete", typ, err)
	}

	return svc.store.SoftDeleteAddon(ctx, existing.ID, deletedBy)
}

// GetConfig returns the addon-config of the project with sensitive keys masked.
func (svc *Service) GetConfig(ctx context.Context, project, typ string) (*Config, error) {
	sensitive, err := svc.registry.sensitiveKeys(typ)
	if err != nil {
		return nil, err
	}

	cfg, err := svc.store.GetAddonConfig(ctx, project, typ)
	if err != nil {
		return nil, err
	}

	cfg.Configs, err = maskConfigs(cfg.Configs, sensitive)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// PutConfig creates or updates the addon-config of the project. Sensitive keys
// sent as the mask placeholder keep their stored value.
func (svc *Service) PutConfig(ctx context.Context, project, typ string, configs json.RawMessage) (*Config, error) {
	sensitive, err := svc.registry.sensitiveKeys(typ)
	if err != nil {
		return nil, err
	}
	if _, isMeta := svc.registry.GetMeta(typ); isMeta {
		if _, err := metaEnabled(configs); err != nil {
			return nil, err
		}
	}

	var stored json.RawMessage
	existing, err := svc.store.GetAddonConfig(ctx, project, typ)
	if err == nil {
		stored = existing.Configs
	} else if !errors.Is(err, errors.ErrNotFound) {
		return nil, err
	}

	restored, err := restoreConfigs(configs, stored, sensitive)
	if err != nil {
		return nil, err
	}

	now := svc.clock()
	cfg := Config{Project: project, Type: typ, Configs: restored, CreatedAt: now, UpdatedAt: now}
	if existing != nil {
		cfg.CreatedAt = existing.CreatedAt
	}
	if err := svc.store.PutAddonConfig(ctx, cfg); err != nil {
		return nil, err
	}

	cfg.Configs, err = maskConfigs(cfg.Configs, sensitive)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// DeleteConfig deletes the addon-config of the project.
func (svc *Service) DeleteConfig(ctx context.Context, project, typ string) error {
	if _, err := svc.registry.sensitiveKeys(typ); err != nil {
		return err
	}
	return svc.store.DeleteAddonConfig(ctx, project, typ)
}

func (svc *Service) loadParent(ctx context.Context, parentURN, typ string) (*resource.Resource, error) {
	parent, err := svc.parents.GetByURN(ctx, parentURN)
	if err != nil {
		return nil, err
	}

	desc, err := svc.descriptors.GetDescriptor(parent.Kind)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(desc.Addons, typ) {
		return nil, errors.ErrInvalid.WithMsgf("add-on type '%s' is not supported by kind '%s'", typ, parent.Kind)
	}
	return parent, nil
}

func (svc *Service) loadConfig(ctx context.Context, drv Driver, project, typ string) (json.RawMessage, error) {
	if !drv.RequiresConfig() {
		return nil, nil
	}

	cfg, err := svc.store.GetAddonConfig(ctx, project, typ)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			return nil, errors.ErrInvalid.WithMsgf("addon-config '%s' is not configured for project '%s'", typ, project)
		}
		return nil, err
	}
	return cfg.Configs, nil
}

// meta computes the meta add-on of the given type on the parent. It returns nil
// when the kind does not support the type or the project addon-config disables it.
func (svc *Service) meta(ctx context.Context, parent resource.Resource, typ string, m Meta) (*Addon, error) {
	desc, err := svc.descriptors.GetDescriptor(parent.Kind)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(desc.Addons, typ) {
		return nil, nil
	}

	cfg, err := svc.store.GetAddonConfig(ctx, parent.Project, typ)
	if err != nil && !errors.Is(err, errors.ErrNotFound) {
		return nil, err
	}
	if cfg != nil {
		enabled, err := metaEnabled(cfg.Configs)
		if err != nil {
			return nil, err
		}
		if !enabled {
			return nil, nil
		}
	}

	return &Addon{
		URN:       GenerateURN(parent.URN, typ),
		ParentURN: parent.URN,
		Type:      typ,
		Spec:      json.RawMessage("{}"),
		Output:    m.Output(parentContext(parent)),
		Status:    StatusCompleted,
		CreatedAt: parent.CreatedAt,
		UpdatedAt: parent.UpdatedAt,
		CreatedBy: parent.CreatedBy,
		UpdatedBy: parent.UpdatedBy,
	}, nil
}

// metaEnabled reads the addon-config of a meta add-on: {"enabled": bool},
// where an absent value means enabled.
func metaEnabled(configs json.RawMessage) (bool, error) {
	var cfg struct {
		Enabled *bool `json:"enabled"`
	}
	dec := json.NewDecoder(bytes.NewReader(configs))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return false, errors.ErrInvalid.WithMsgf("invalid meta add-on config: %s", err.Error())
	}
	return cfg.Enabled == nil || *cfg.Enabled, nil
}

func parentContext(res resource.Resource) ParentContext {
	return ParentContext{
		URN:     res.URN,
		Kind:    res.Kind,
		Project: res.Project,
		Name:    res.Name,
		Output:  res.State.Output,
	}
}

func asInvalid(err error) error {
	var e errors.Error
	if errors.As(err, &e) {
		return e
	}
	return errors.ErrInvalid.WithMsgf("%s", err.Error())
}

// failed keeps categorised driver errors and reports others as internal,
// with the driver's message so that operators can act on it.
func failed(op, typ string, err error) error {
	var e errors.Error
	if errors.As(err, &e) {
		return e
	}
	return errors.ErrInternal.WithMsgf("failed to %s add-on '%s': %s", op, typ, err.Error())
}
