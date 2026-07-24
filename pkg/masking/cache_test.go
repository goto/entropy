package masking

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeLookup struct {
	configs map[string]json.RawMessage
	calls   map[string]int
	err     error
}

func (f *fakeLookup) ModuleConfigs(_ context.Context, urn string) (json.RawMessage, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[urn]++
	if f.err != nil {
		return nil, f.err
	}
	cfg, ok := f.configs[urn]
	if !ok {
		return nil, errors.New("not found")
	}
	return cfg, nil
}

func TestConfigCache_PathsForAndCache(t *testing.T) {
	urn := "orn:entropy:module:my-project:firehose"
	lk := &fakeLookup{configs: map[string]json.RawMessage{
		urn: json.RawMessage(`{"sensitive_config":["env_variables.PWD","env_variables.*"]}`),
	}}
	c := NewConfigCache(lk)

	paths, err := c.PathsFor(context.Background(), "firehose", "my-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "env_variables.PWD" {
		t.Errorf("unexpected paths: %v", paths)
	}

	// Second call for same (kind, project) must hit the cache.
	if _, err := c.PathsFor(context.Background(), "firehose", "my-project"); err != nil {
		t.Fatal(err)
	}
	if lk.calls[urn] != 1 {
		t.Errorf("expected 1 lookup, got %d", lk.calls[urn])
	}
}

func TestConfigCache_MissingModulePropagatesError(t *testing.T) {
	c := NewConfigCache(&fakeLookup{configs: map[string]json.RawMessage{}})
	if _, err := c.PathsFor(context.Background(), "unknown", "p"); err == nil {
		t.Error("expected error for missing module")
	}
}

func TestConfigCache_EvictForcesReResolve(t *testing.T) {
	urn := "orn:entropy:module:my-project:firehose"
	lk := &fakeLookup{configs: map[string]json.RawMessage{
		urn: json.RawMessage(`{"sensitive_config":["env_variables.PWD"]}`),
	}}
	c := NewConfigCache(lk)

	if _, err := c.PathsFor(context.Background(), "firehose", "my-project"); err != nil {
		t.Fatal(err)
	}
	if lk.calls[urn] != 1 {
		t.Fatalf("expected 1 lookup, got %d", lk.calls[urn])
	}

	c.Evict("firehose", "my-project")

	lk.configs[urn] = json.RawMessage(`{"sensitive_config":["env_variables.PWD","env_variables.TOKEN"]}`)
	paths, err := c.PathsFor(context.Background(), "firehose", "my-project")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Errorf("expected re-resolved paths after evict, got %v", paths)
	}
	if lk.calls[urn] != 2 {
		t.Errorf("expected 2 lookups after evict, got %d", lk.calls[urn])
	}
}

func TestPathsFromConfigs_MissingKey(t *testing.T) {
	paths, err := PathsFromConfigs(json.RawMessage(`{"namespace":{"default":"x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if paths != nil {
		t.Errorf("expected nil paths, got %v", paths)
	}
}
