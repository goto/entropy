package dagger

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
)

var frozenTime = time.Unix(1679668743, 0)

func TestDaggerDriver_Plan(t *testing.T) {
	t.Parallel()

	t.Run("Create_Valid", func(t *testing.T) {
		t.Parallel()

		dr := newTestDriver()
		exr := testExpandedResource(nil)

		got, err := dr.Plan(context.Background(), exr, module.ActionRequest{
			Name:   module.CreateAction,
			Params: modules.MustJSON(validConfigMap()),
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, resource.StatusPending, got.State.Status)
		require.NotNil(t, got.State.NextSyncAt)
		assert.Equal(t, frozenTime, *got.State.NextSyncAt)
		assert.Equal(t, JobStateRunning, got.Labels[labelJobState])
		assert.Equal(t, StateDeployed, got.Labels[labelState])

		cfg := readTestConfig(t, got.Spec.Configs)
		assert.Equal(t, StateDeployed, cfg.State)
		assert.Equal(t, JobStateRunning, cfg.JobState)
		assert.Equal(t, "dagger", cfg.Namespace)
		assert.Equal(t, dr.conf.JarURI, cfg.JarURI)

		md := readTestTransientData(t, got.State.ModuleData)
		assert.Equal(t, []string{stepReleaseCreate}, md.PendingSteps)
	})

	t.Run("Update_Valid", func(t *testing.T) {
		t.Parallel()

		dr := newTestDriver()
		exr := testExpandedResource(modules.MustJSON(validConfigMap()))

		update := validConfigMap()
		update["replicas"] = 3

		got, err := dr.Plan(context.Background(), exr, module.ActionRequest{
			Name:   module.UpdateAction,
			Params: modules.MustJSON(update),
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, resource.StatusPending, got.State.Status)
		assert.Equal(t, JobStateRunning, got.Labels[labelJobState])
		assert.Equal(t, StateDeployed, got.Labels[labelState])

		cfg := readTestConfig(t, got.Spec.Configs)
		assert.Equal(t, 3, cfg.Replicas)
		assert.Equal(t, StateDeployed, cfg.State)
		assert.Equal(t, JobStateRunning, cfg.JobState)

		md := readTestTransientData(t, got.State.ModuleData)
		assert.Equal(t, []string{stepReleaseUpdate}, md.PendingSteps)
	})

	t.Run("Stop_Valid", func(t *testing.T) {
		t.Parallel()

		dr := newTestDriver()
		exr := testExpandedResource(modules.MustJSON(validConfigMap()))

		got, err := dr.Plan(context.Background(), exr, module.ActionRequest{Name: StopAction})
		require.NoError(t, err)
		require.NotNil(t, got)

		cfg := readTestConfig(t, got.Spec.Configs)
		assert.Equal(t, StateUserStopped, cfg.State)
		assert.Equal(t, JobStateSuspended, cfg.JobState)
		assert.Equal(t, JobStateSuspended, got.Labels[labelJobState])
		assert.Equal(t, StateUserStopped, got.Labels[labelState])

		md := readTestTransientData(t, got.State.ModuleData)
		assert.Equal(t, []string{stepReleaseUpdate}, md.PendingSteps)
	})

	t.Run("Start_Valid", func(t *testing.T) {
		t.Parallel()

		dr := newTestDriver()
		base := validConfigMap()
		base["state"] = StateUserStopped
		base["job_state"] = JobStateSuspended
		exr := testExpandedResource(modules.MustJSON(base))

		got, err := dr.Plan(context.Background(), exr, module.ActionRequest{Name: StartAction})
		require.NoError(t, err)
		require.NotNil(t, got)

		cfg := readTestConfig(t, got.Spec.Configs)
		assert.Equal(t, StateDeployed, cfg.State)
		assert.Equal(t, JobStateRunning, cfg.JobState)
		assert.Equal(t, dr.conf.JarURI, cfg.JarURI)
		assert.Equal(t, JobStateRunning, got.Labels[labelJobState])
		assert.Equal(t, StateDeployed, got.Labels[labelState])

		md := readTestTransientData(t, got.State.ModuleData)
		assert.Equal(t, []string{stepReleaseUpdate}, md.PendingSteps)
	})

	t.Run("Reset_Invalid", func(t *testing.T) {
		t.Parallel()

		dr := newTestDriver()
		exr := testExpandedResource(modules.MustJSON(validConfigMap()))

		got, err := dr.Plan(context.Background(), exr, module.ActionRequest{
			Name: ResetAction,
			Params: modules.MustJSON(map[string]any{
				"to": "invalid_value",
			}),
		})
		require.Error(t, err)
		assert.Nil(t, got)
		assert.True(t, errors.Is(err, errors.ErrInvalid), "wantErr=%v gotErr=%v", errors.ErrInvalid, err)
	})

	t.Run("Reset_Valid", func(t *testing.T) {
		t.Parallel()

		dr := newTestDriver()
		exr := testExpandedResource(modules.MustJSON(validConfigMap()))

		got, err := dr.Plan(context.Background(), exr, module.ActionRequest{
			Name: ResetAction,
			Params: modules.MustJSON(map[string]any{
				"to": "earliest",
			}),
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, resource.StatusPending, got.State.Status)
		require.NotNil(t, got.State.NextSyncAt)
		assert.Equal(t, frozenTime, *got.State.NextSyncAt)

		cfg := readTestConfig(t, got.Spec.Configs)
		assert.Equal(t, "earliest", cfg.ResetOffset)

		md := readTestTransientData(t, got.State.ModuleData)
		assert.Equal(t, "earliest", md.ResetOffsetTo)
		assert.Equal(t, []string{stepKafkaReset}, md.PendingSteps)
	})

	t.Run("Update_DisableAutoUpdateStencilVersion", func(t *testing.T) {
		t.Parallel()

		dr := newTestDriver()

		existingConfig := validConfigMap()
		existingConfig["env_variables"] = map[string]string{
			"SINK_TYPE":                           "INFLUX",
			"DISABLE_AUTO_UPDATE_STENCIL_VERSION": "true",
			"SCHEMA_REGISTRY_STENCIL_URLS":        "http://stencil.old/v1",
		}
		exr := testExpandedResource(modules.MustJSON(existingConfig))

		updateConfig := validConfigMap()
		updateConfig["env_variables"] = map[string]string{
			"SINK_TYPE":                           "INFLUX",
			"DISABLE_AUTO_UPDATE_STENCIL_VERSION": "true",
			"SCHEMA_REGISTRY_STENCIL_URLS":        "http://stencil.new/v2",
		}

		got, err := dr.Plan(context.Background(), exr, module.ActionRequest{
			Name:   module.UpdateAction,
			Params: modules.MustJSON(updateConfig),
		})
		require.NoError(t, err)
		require.NotNil(t, got)

		cfg := readTestConfig(t, got.Spec.Configs)
		assert.Equal(t, "http://stencil.old/v1", cfg.EnvVariables[KeySchemaRegistryStencilURLs],
			"stencil URLs should be preserved from existing config when auto update is disabled")
	})
}

func newTestDriver() *daggerDriver {
	conf := defaultDriverConf
	conf.ChartValues.ImageRepository = "gotocompany/dagger"
	conf.ChartValues.ImageTag = "latest"
	conf.ChartValues.ImagePullPolicy = "IfNotPresent"
	conf.JarURI = "s3://jar-uri/dagger.jar"
	conf.KubeDeployTimeout = 15
	conf.EnvVariables = map[string]string{
		SourceKafkaConsumerConfigAutoOffsetReset:  "latest",
		SourceKafkaConsumerConfigAutoCommitEnable: "true",
		SourceKafkaConsumerConfigBootstrapServers: "localhost:9092",
	}

	return &daggerDriver{
		conf:    conf,
		timeNow: func() time.Time { return frozenTime },
		consumerReset: func(_ context.Context, cfg Config, resetTo string) []Source {
			for i := range cfg.Source {
				cfg.Source[i].SourceKafkaConsumerConfigAutoOffsetReset = resetTo
			}
			return cfg.Source
		},
	}
}

func testExpandedResource(configs []byte) module.ExpandedResource {
	if configs == nil {
		configs = modules.MustJSON(validConfigMap())
	}

	return module.ExpandedResource{
		Resource: resource.Resource{
			URN:     "urn:goto:entropy:foo:dg1",
			Kind:    "dagger",
			Name:    "dg1",
			Project: "foo",
			Labels:  map[string]string{},
			Spec: resource.Spec{
				Configs: configs,
			},
			State: resource.State{
				Status: resource.StatusCompleted,
				Output: modules.MustJSON(Output{Namespace: "dagger"}),
			},
		},
		Dependencies: map[string]module.ResolvedDependency{
			keyFlinkDependency: {
				Kind: "flink",
				Output: modules.MustJSON(flink.Output{
					KubeNamespace: "dagger",
					Influx: flink.Influx{
						URL:          "http://influx.local",
						Username:     "user",
						Password:     "pass",
						DatabaseName: "db",
					},
				}),
			},
		},
	}
}

func validConfigMap() map[string]any {
	return map[string]any{
		"replicas":  1,
		"team":      "platform",
		"sink_type": "INFLUX",
		"state":     StateDeployed,
		"job_state": JobStateRunning,
		"chart_values": map[string]any{
			"chart_version":     "0.1.0",
			"image_repository":  "gotocompany/dagger",
			"image_tag":         "latest",
			"image_pull_policy": "IfNotPresent",
		},
		"env_variables": map[string]string{
			"SINK_TYPE": "INFLUX",
		},
		"source": []map[string]any{
			{
				"INPUT_SCHEMA_PROTO_CLASS":                       "com.foo.Bar",
				"SOURCE_KAFKA_NAME":                              "source-a",
				"SOURCE_KAFKA_TOPIC_NAMES":                       "topic-a",
				"SOURCE_KAFKA_CONSUMER_CONFIG_GROUP_ID":          "dagger-deployment-x-0001",
				"SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET": "latest",
				"SOURCE_KAFKA_CONSUMER_CONFIG_BOOTSTRAP_SERVERS": "localhost:9092",
			},
		},
		"sink": map[string]any{
			"SINK_INFLUX_MEASUREMENT_NAME": "measurement-a",
		},
	}
}

func readTestConfig(t *testing.T, data []byte) Config {
	t.Helper()

	var cfg Config
	require.NoError(t, json.Unmarshal(data, &cfg))
	return cfg
}

func readTestTransientData(t *testing.T, data []byte) transientData {
	t.Helper()

	var md transientData
	require.NoError(t, json.Unmarshal(data, &md))
	return md
}
