package dagger

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
)

func csvExpandedResource() module.ExpandedResource {
	return module.ExpandedResource{
		Resource: resource.Resource{
			URN:     "orn:entropy:dagger:test:csv-test",
			Kind:    "dagger",
			Name:    "csv-test",
			Project: "test-project",
		},
		Dependencies: map[string]module.ResolvedDependency{
			keyFlinkDependency: {
				Kind:   "flink",
				Output: json.RawMessage(`{"kube_namespace":"test-ns"}`),
			},
		},
	}
}

func csvConfigJSON(t *testing.T, sink map[string]string) json.RawMessage {
	t.Helper()
	cfg := map[string]any{
		"replicas":  1,
		"team":      "test-team",
		"sink_type": SinkTypeCSV,
		"source": []map[string]any{
			{"SOURCE_KAFKA_CONSUMER_CONFIG_GROUP_ID": "test-0001"},
		},
		"sink":          sink,
		"env_variables": map[string]string{keySinkType: SinkTypeCSV},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	return b
}

func TestReadConfigCSVSink(t *testing.T) {
	sink := map[string]string{
		keySinkCsvBasePath:  "gs://bucket/some-folder",
		keySinkCsvWriteMode: "OVERWRITE",
	}

	cfg, err := readConfig(csvExpandedResource(), csvConfigJSON(t, sink), driverConf{})
	if err != nil {
		t.Fatalf("readConfig returned error: %v", err)
	}

	if got := cfg.EnvVariables[keySinkType]; got != SinkTypeCSV {
		t.Errorf("SINK_TYPE = %q, want %q", got, SinkTypeCSV)
	}
	if got := cfg.EnvVariables[keySinkCsvBasePath]; got != "gs://bucket/some-folder" {
		t.Errorf("SINK_CSV_BASE_PATH = %q, want %q", got, "gs://bucket/some-folder")
	}
	if got := cfg.EnvVariables[keySinkCsvWriteMode]; got != "OVERWRITE" {
		t.Errorf("SINK_CSV_WRITE_MODE = %q, want %q", got, "OVERWRITE")
	}

	// Optional keys that were not provided must NOT be emitted, so the Dagger app's
	// own defaults (delimiter, timezone, header, ...) are not clobbered with empty strings.
	if v, ok := cfg.EnvVariables[keySinkCsvDelimiter]; ok {
		t.Errorf("SINK_CSV_DELIMITER should be absent, got %q", v)
	}
	if v, ok := cfg.EnvVariables[keySinkCsvPartitionTimezone]; ok {
		t.Errorf("SINK_CSV_PARTITION_TIMEZONE should be absent, got %q", v)
	}
}

func TestReadConfigCSVSinkRequiresBasePath(t *testing.T) {
	cfg, err := readConfig(csvExpandedResource(), csvConfigJSON(t, map[string]string{}), driverConf{})
	if err == nil {
		t.Fatalf("expected error for missing SINK_CSV_BASE_PATH, got nil (cfg=%+v)", cfg)
	}
	if !strings.Contains(err.Error(), keySinkCsvBasePath) {
		t.Errorf("error = %q, want it to mention %q", err.Error(), keySinkCsvBasePath)
	}
}

func TestValidateConfigSinkTypeEnum(t *testing.T) {
	// CSV must now be accepted by the embedded JSON schema.
	if err := validateConfig(csvConfigJSON(t, map[string]string{keySinkCsvBasePath: "gs://b/f"})); err != nil {
		t.Errorf("validateConfig rejected CSV sink_type: %v", err)
	}

	// An unknown sink type must still be rejected by the schema.
	bad := map[string]any{
		"replicas":      1,
		"team":          "test-team",
		"sink_type":     "PARQUET",
		"source":        []map[string]any{{"SOURCE_KAFKA_CONSUMER_CONFIG_GROUP_ID": "test-0001"}},
		"sink":          map[string]string{},
		"env_variables": map[string]string{keySinkType: "PARQUET"},
	}
	b, err := json.Marshal(bad)
	if err != nil {
		t.Fatalf("marshal bad config: %v", err)
	}
	if err := validateConfig(json.RawMessage(b)); err == nil {
		t.Error("validateConfig accepted unknown sink_type PARQUET, want rejection")
	}
}
