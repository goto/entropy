package addon_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/addon"
	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

const (
	parentURN = "orn:entropy:firehose:team-a:orders"
	typLogs   = "logs"
)

var frozenTime = time.Unix(1700000000, 0)

func TestService_Apply(t *testing.T) {
	t.Parallel()

	completed := &resource.Resource{
		URN: parentURN, Kind: "firehose", Project: "team-a", Name: "orders",
		State: resource.State{Status: resource.StatusCompleted, Output: json.RawMessage(`{"namespace":"ns"}`)},
	}
	pending := *completed
	pending.State.Status = resource.StatusPending

	version := func(v int64) *int64 { return &v }

	tests := []struct {
		name        string
		parent      *resource.Resource
		existing    *addon.Addon
		noConfig    bool
		driver      *fakeDriver
		req         addon.ApplyRequest
		wantErr     error
		wantErrMsg  string
		wantStored  *addon.Addon
		wantApplied bool
	}{
		{
			name:        "CreatesOnSuccess",
			parent:      completed,
			driver:      &fakeDriver{output: `{"id":"x"}`},
			req:         addon.ApplyRequest{Spec: json.RawMessage(`{"labels":{"a":"b"}}`)},
			wantApplied: true,
			wantStored: &addon.Addon{
				URN: parentURN + ":logs", Spec: json.RawMessage(`{"labels":{"a":"b"}}`),
				Output: json.RawMessage(`{"id":"x"}`), Status: addon.StatusCompleted, SpecVersion: 1,
			},
		},
		{
			name:        "DefaultsEmptySpec",
			parent:      completed,
			driver:      &fakeDriver{},
			req:         addon.ApplyRequest{},
			wantApplied: true,
			wantStored: &addon.Addon{
				URN: parentURN + ":logs", Spec: json.RawMessage(`{}`),
				Output: json.RawMessage(`{}`), Status: addon.StatusCompleted, SpecVersion: 1,
			},
		},
		{
			name:     "UpdatesAndBumpsVersion",
			parent:   completed,
			existing: &addon.Addon{Spec: json.RawMessage(`{}`), Output: json.RawMessage(`{}`), Status: addon.StatusCompleted, SpecVersion: 3},
			driver:   &fakeDriver{output: `{"id":"y"}`},
			req:      addon.ApplyRequest{Spec: json.RawMessage(`{"labels":{}}`), SpecVersion: version(3)},
			wantStored: &addon.Addon{
				URN: parentURN + ":logs", Spec: json.RawMessage(`{"labels":{}}`),
				Output: json.RawMessage(`{"id":"y"}`), Status: addon.StatusCompleted, SpecVersion: 4,
			},
			wantApplied: true,
		},
		{
			name:        "FirstApplyFailureStoresError",
			parent:      completed,
			driver:      &fakeDriver{applyErr: fmt.Errorf("backend down")},
			req:         addon.ApplyRequest{Spec: json.RawMessage(`{"labels":{}}`)},
			wantErr:     errors.ErrInternal,
			wantErrMsg:  "failed to apply add-on 'logs': backend down",
			wantApplied: true,
			wantStored: &addon.Addon{
				URN: parentURN + ":logs", Spec: json.RawMessage(`{"labels":{}}`),
				Output: json.RawMessage(`{}`), Status: addon.StatusError, LastError: "backend down", SpecVersion: 1,
			},
		},
		{
			name:        "UpdateFailureKeepsPreviousSpec",
			parent:      completed,
			existing:    &addon.Addon{Spec: json.RawMessage(`{}`), Output: json.RawMessage(`{"id":"x"}`), Status: addon.StatusCompleted, SpecVersion: 2},
			driver:      &fakeDriver{applyErr: fmt.Errorf("backend down")},
			req:         addon.ApplyRequest{Spec: json.RawMessage(`{"labels":{"new":"1"}}`)},
			wantErr:     errors.ErrInternal,
			wantApplied: true,
			wantStored: &addon.Addon{
				URN: parentURN + ":logs", Spec: json.RawMessage(`{}`),
				Output: json.RawMessage(`{"id":"x"}`), Status: addon.StatusError, LastError: "backend down", SpecVersion: 2,
			},
		},
		{
			name:     "SpecVersionConflict",
			parent:   completed,
			existing: &addon.Addon{Spec: json.RawMessage(`{}`), Status: addon.StatusCompleted, SpecVersion: 2},
			driver:   &fakeDriver{},
			req:      addon.ApplyRequest{SpecVersion: version(1)},
			wantErr:  errors.ErrConflict,
		},
		{
			name:    "SpecVersionOnMissingAddon",
			parent:  completed,
			driver:  &fakeDriver{},
			req:     addon.ApplyRequest{SpecVersion: version(1)},
			wantErr: errors.ErrConflict,
		},
		{
			name:       "ParentNotCompleted",
			parent:     &pending,
			driver:     &fakeDriver{requiresCompleted: true},
			wantErr:    errors.ErrInvalid,
			wantErrMsg: "parent must be in STATUS_COMPLETED before applying add-on 'logs'",
		},
		{
			name:    "ParentNotFound",
			driver:  &fakeDriver{},
			wantErr: errors.ErrNotFound,
		},
		{
			name:       "TypeNotSupportedByKind",
			parent:     &resource.Resource{URN: parentURN, Kind: "kafka", Project: "team-a"},
			driver:     &fakeDriver{},
			wantErr:    errors.ErrInvalid,
			wantErrMsg: "add-on type 'logs' is not supported by kind 'kafka'",
		},
		{
			name:    "InvalidSpec",
			parent:  completed,
			driver:  &fakeDriver{validateErr: fmt.Errorf("bad spec")},
			wantErr: errors.ErrInvalid,
		},
		{
			name:       "MissingAddonConfig",
			parent:     completed,
			noConfig:   true,
			driver:     &fakeDriver{requiresConfig: true},
			wantErr:    errors.ErrInvalid,
			wantErrMsg: "addon-config 'logs' is not configured for project 'team-a'",
		},
		{
			name:        "NoConfigNeeded",
			parent:      completed,
			noConfig:    true,
			driver:      &fakeDriver{},
			wantApplied: true,
			wantStored: &addon.Addon{
				URN: parentURN + ":logs", Spec: json.RawMessage(`{}`),
				Output: json.RawMessage(`{}`), Status: addon.StatusCompleted, SpecVersion: 1,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newMemStore()
			if !tt.noConfig {
				store.configs[configKey("team-a", typLogs)] = addon.Config{Project: "team-a", Type: typLogs, Configs: json.RawMessage(`{"endpoint":"http://localhost"}`)}
			}
			if tt.existing != nil {
				a := *tt.existing
				a.URN, a.ParentURN, a.Type = parentURN+":logs", parentURN, typLogs
				require.NoError(t, store.CreateAddon(context.Background(), &a))
			}
			svc := newService(t, store, tt.parent, tt.driver)

			req := tt.req
			req.ParentURN, req.Type, req.UserID = parentURN, typLogs, "alice"
			got, err := svc.Apply(context.Background(), req)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
				if tt.wantErrMsg != "" {
					assert.Equal(t, tt.wantErrMsg, errors.E(err).Message)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantStored.SpecVersion, got.SpecVersion)
			}
			assert.Equal(t, tt.wantApplied, tt.driver.applied)
			if tt.driver.requiresConfig && tt.driver.applied {
				assert.JSONEq(t, `{"endpoint":"http://localhost"}`, string(tt.driver.config))
			}

			stored, err := store.GetAddon(context.Background(), parentURN, typLogs)
			if tt.wantStored == nil {
				if tt.existing == nil {
					assert.ErrorIs(t, err, errors.ErrNotFound)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStored.URN, stored.URN)
			assert.JSONEq(t, string(tt.wantStored.Spec), string(stored.Spec))
			assert.JSONEq(t, string(tt.wantStored.Output), string(stored.Output))
			assert.Equal(t, tt.wantStored.Status, stored.Status)
			assert.Equal(t, tt.wantStored.LastError, stored.LastError)
			assert.Equal(t, tt.wantStored.SpecVersion, stored.SpecVersion)
			assert.Equal(t, "alice", stored.UpdatedBy)
		})
	}
}

func TestService_ApplyTimeout(t *testing.T) {
	t.Parallel()

	store := newMemStore()
	parent := &resource.Resource{URN: parentURN, Kind: "firehose", Project: "team-a", State: resource.State{Status: resource.StatusCompleted}}
	drv := &fakeDriver{waitForCtx: true}
	svc := addon.NewService(store, registryWith(t, drv), fakeParents{parent}, fakeDescriptors{}, func() time.Time { return frozenTime }, 10*time.Millisecond)

	_, err := svc.Apply(context.Background(), addon.ApplyRequest{ParentURN: parentURN, Type: typLogs, UserID: "alice"})
	require.Error(t, err)

	stored, err := store.GetAddon(context.Background(), parentURN, typLogs)
	require.NoError(t, err)
	assert.Equal(t, addon.StatusError, stored.Status)
	assert.Contains(t, stored.LastError, "deadline exceeded")
}

func TestService_Delete(t *testing.T) {
	t.Parallel()

	parent := &resource.Resource{URN: parentURN, Kind: "firehose", Project: "team-a"}

	t.Run("CallsDriverThenSoftDeletes", func(t *testing.T) {
		t.Parallel()

		store := newMemStore()
		a := addon.Addon{URN: parentURN + ":logs", ParentURN: parentURN, Type: typLogs, Spec: json.RawMessage(`{"labels":{}}`)}
		require.NoError(t, store.CreateAddon(context.Background(), &a))
		drv := &fakeDriver{}
		svc := newService(t, store, parent, drv)

		require.NoError(t, svc.Delete(context.Background(), parentURN, typLogs, "bob"))
		assert.True(t, drv.deleted)
		assert.JSONEq(t, `{"labels":{}}`, string(drv.spec))
		assert.Equal(t, "bob", store.deletedBy[a.ID])

		_, err := store.GetAddon(context.Background(), parentURN, typLogs)
		assert.ErrorIs(t, err, errors.ErrNotFound)
	})

	t.Run("DriverFailureKeepsRow", func(t *testing.T) {
		t.Parallel()

		store := newMemStore()
		a := addon.Addon{URN: parentURN + ":logs", ParentURN: parentURN, Type: typLogs, Spec: json.RawMessage(`{}`)}
		require.NoError(t, store.CreateAddon(context.Background(), &a))
		svc := newService(t, store, parent, &fakeDriver{deleteErr: fmt.Errorf("backend down")})

		err := svc.Delete(context.Background(), parentURN, typLogs, "bob")
		assert.ErrorIs(t, err, errors.ErrInternal)

		_, err = store.GetAddon(context.Background(), parentURN, typLogs)
		assert.NoError(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()

		drv := &fakeDriver{}
		svc := newService(t, newMemStore(), parent, drv)
		err := svc.Delete(context.Background(), parentURN, typLogs, "bob")
		assert.ErrorIs(t, err, errors.ErrNotFound)
		assert.False(t, drv.deleted)
	})
}

func TestService_Configs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newMemStore()
	svc := newService(t, store, nil, &fakeDriver{sensitive: []string{"token"}})

	got, err := svc.PutConfig(ctx, "team-a", typLogs, json.RawMessage(`{"endpoint":"http://a","token":"s3cret"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"endpoint":"http://a","token":"****"}`, string(got.Configs))
	assert.JSONEq(t, `{"endpoint":"http://a","token":"s3cret"}`, string(store.configs[configKey("team-a", typLogs)].Configs))

	got, err = svc.GetConfig(ctx, "team-a", typLogs)
	require.NoError(t, err)
	assert.JSONEq(t, `{"endpoint":"http://a","token":"****"}`, string(got.Configs))

	// round-tripping the masked value keeps the stored secret.
	_, err = svc.PutConfig(ctx, "team-a", typLogs, json.RawMessage(`{"endpoint":"http://b","token":"****"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"endpoint":"http://b","token":"s3cret"}`, string(store.configs[configKey("team-a", typLogs)].Configs))

	// masked value without anything stored is dropped.
	_, err = svc.PutConfig(ctx, "team-b", typLogs, json.RawMessage(`{"endpoint":"http://c","token":"****"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"endpoint":"http://c"}`, string(store.configs[configKey("team-b", typLogs)].Configs))

	_, err = svc.PutConfig(ctx, "team-a", typLogs, json.RawMessage(`[1]`))
	assert.ErrorIs(t, err, errors.ErrInvalid)

	_, err = svc.PutConfig(ctx, "team-a", "unknown", json.RawMessage(`{}`))
	assert.ErrorIs(t, err, errors.ErrInvalid)

	require.NoError(t, svc.DeleteConfig(ctx, "team-a", typLogs))
	_, err = svc.GetConfig(ctx, "team-a", typLogs)
	assert.ErrorIs(t, err, errors.ErrNotFound)
}

func newService(t *testing.T, store *memStore, parent *resource.Resource, drv *fakeDriver) *addon.Service {
	t.Helper()
	return addon.NewService(store, registryWith(t, drv), fakeParents{parent}, fakeDescriptors{},
		func() time.Time { return frozenTime }, time.Second)
}

func registryWith(t *testing.T, drv *fakeDriver) *addon.Registry {
	t.Helper()
	reg := &addon.Registry{}
	require.NoError(t, reg.Register(typLogs, drv))
	return reg
}

type fakeParents struct{ res *resource.Resource }

func (f fakeParents) GetByURN(_ context.Context, urn string) (*resource.Resource, error) {
	if f.res == nil || f.res.URN != urn {
		return nil, errors.ErrNotFound
	}
	res := *f.res
	return &res, nil
}

type fakeDescriptors struct{}

func (fakeDescriptors) GetDescriptor(kind string) (module.Descriptor, error) {
	if kind == "firehose" {
		return module.Descriptor{Kind: kind, Addons: []string{typLogs}}, nil
	}
	return module.Descriptor{Kind: kind}, nil
}

type fakeDriver struct {
	requiresCompleted bool
	requiresConfig    bool
	sensitive         []string
	validateErr       error
	output            string
	applyErr          error
	deleteErr         error
	waitForCtx        bool

	applied bool
	deleted bool
	config  json.RawMessage
	spec    json.RawMessage
}

func (d *fakeDriver) Validate(json.RawMessage) error { return d.validateErr }
func (d *fakeDriver) RequiresParentCompleted() bool  { return d.requiresCompleted }
func (d *fakeDriver) RequiresConfig() bool           { return d.requiresConfig }
func (d *fakeDriver) SensitiveConfigKeys() []string  { return d.sensitive }

func (d *fakeDriver) Apply(ctx context.Context, _ addon.ParentContext, config, spec json.RawMessage) (json.RawMessage, error) {
	d.applied, d.config, d.spec = true, config, spec
	if d.waitForCtx {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if d.applyErr != nil {
		return nil, d.applyErr
	}
	return json.RawMessage(d.output), nil
}

func (d *fakeDriver) Delete(_ context.Context, _ addon.ParentContext, config, spec json.RawMessage) error {
	d.deleted, d.config, d.spec = true, config, spec
	return d.deleteErr
}

type memStore struct {
	nextID    int64
	addons    map[int64]addon.Addon
	deletedBy map[int64]string
	configs   map[string]addon.Config
}

func newMemStore() *memStore {
	return &memStore{addons: map[int64]addon.Addon{}, deletedBy: map[int64]string{}, configs: map[string]addon.Config{}}
}

func configKey(project, typ string) string { return project + "/" + typ }

func (m *memStore) GetAddon(_ context.Context, parentURN, typ string) (*addon.Addon, error) {
	for id, a := range m.addons {
		if _, deleted := m.deletedBy[id]; !deleted && a.ParentURN == parentURN && a.Type == typ {
			return &a, nil
		}
	}
	return nil, errors.ErrNotFound
}

func (m *memStore) ListAddons(_ context.Context, parentURN string) ([]addon.Addon, error) {
	var res []addon.Addon
	for id, a := range m.addons {
		if _, deleted := m.deletedBy[id]; !deleted && a.ParentURN == parentURN {
			res = append(res, a)
		}
	}
	return res, nil
}

func (m *memStore) CreateAddon(ctx context.Context, a *addon.Addon) error {
	if _, err := m.GetAddon(ctx, a.ParentURN, a.Type); err == nil {
		return errors.ErrConflict
	}
	m.nextID++
	a.ID = m.nextID
	m.addons[a.ID] = *a
	return nil
}

func (m *memStore) UpdateAddon(_ context.Context, a addon.Addon, expectedVersion int64) error {
	cur, found := m.addons[a.ID]
	if _, deleted := m.deletedBy[a.ID]; !found || deleted || cur.SpecVersion != expectedVersion {
		return errors.ErrConflict
	}
	m.addons[a.ID] = a
	return nil
}

func (m *memStore) SoftDeleteAddon(_ context.Context, id int64, deletedBy string) error {
	if _, deleted := m.deletedBy[id]; deleted {
		return errors.ErrNotFound
	}
	m.deletedBy[id] = deletedBy
	return nil
}

func (m *memStore) GetAddonConfig(_ context.Context, project, typ string) (*addon.Config, error) {
	cfg, found := m.configs[configKey(project, typ)]
	if !found {
		return nil, errors.ErrNotFound
	}
	return &cfg, nil
}

func (m *memStore) PutAddonConfig(_ context.Context, cfg addon.Config) error {
	m.configs[configKey(cfg.Project, cfg.Type)] = cfg
	return nil
}

func (m *memStore) DeleteAddonConfig(_ context.Context, project, typ string) error {
	if _, found := m.configs[configKey(project, typ)]; !found {
		return errors.ErrNotFound
	}
	delete(m.configs, configKey(project, typ))
	return nil
}
