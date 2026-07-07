package firehose

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
)

func testDriverConf() driverConf {
	return driverConf{
		Namespace: map[string]string{
			defaultKey: "firehose",
		},
		ChartValues: ChartValues{
			ImageRepository: imageRepo,
			ImageTag:        "latest",
			ChartVersion:    "0.1.3",
			ImagePullPolicy: "IfNotPresent",
		},
		RequestsAndLimits: map[string]RequestsAndLimits{
			defaultKey: {
				Limits:   UsageSpec{CPU: "200m", Memory: "512Mi"},
				Requests: UsageSpec{CPU: "200m", Memory: "512Mi"},
			},
		},
	}
}

func baseEnvVariables() map[string]string {
	return map[string]string{
		"SINK_TYPE":                "LOG",
		"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
		"SOURCE_KAFKA_BROKERS":     "localhost:9092",
		"SOURCE_KAFKA_TOPIC":       "foo-log",
	}
}

func Test_readConfig_limitsAndRequests(t *testing.T) {
	t.Parallel()

	res := resource.Resource{
		URN:     "urn:goto:entropy:foo:fh1",
		Kind:    "firehose",
		Name:    "fh1",
		Project: "foo",
	}

	t.Run("FullOverride", func(t *testing.T) {
		conf := modules.MustJSON(map[string]any{
			"replicas":      1,
			"env_variables": baseEnvVariables(),
			"limits":        map[string]any{"cpu": "500m", "memory": "2048Mi"},
			"requests":      map[string]any{"cpu": "400m", "memory": "1024Mi"},
		})

		got, err := readConfig(res, conf, testDriverConf())
		require.NoError(t, err)
		assert.Equal(t, UsageSpec{CPU: "500m", Memory: "2048Mi"}, got.Limits)
		assert.Equal(t, UsageSpec{CPU: "400m", Memory: "1024Mi"}, got.Requests)
	})

	t.Run("PartialOverride_InheritsDriverDefault", func(t *testing.T) {
		conf := modules.MustJSON(map[string]any{
			"replicas":      1,
			"env_variables": baseEnvVariables(),
			"limits":        map[string]any{"cpu": "500m"},
		})

		got, err := readConfig(res, conf, testDriverConf())
		require.NoError(t, err)
		// cpu is overridden, memory falls back to the driver default.
		assert.Equal(t, UsageSpec{CPU: "500m", Memory: "512Mi"}, got.Limits)
		// requests entirely omitted -> both fields from driver default.
		assert.Equal(t, UsageSpec{CPU: "200m", Memory: "512Mi"}, got.Requests)
	})

	t.Run("Omitted_UsesDriverDefault", func(t *testing.T) {
		conf := modules.MustJSON(map[string]any{
			"replicas":      1,
			"env_variables": baseEnvVariables(),
		})

		got, err := readConfig(res, conf, testDriverConf())
		require.NoError(t, err)
		assert.Equal(t, UsageSpec{CPU: "200m", Memory: "512Mi"}, got.Limits)
		assert.Equal(t, UsageSpec{CPU: "200m", Memory: "512Mi"}, got.Requests)
	})

	invalid := []struct {
		name  string
		limit any
	}{
		{name: "CPUAsNumber", limit: map[string]any{"cpu": 500}},
		{name: "UnknownKey", limit: map[string]any{"cpus": "500m"}},
		{name: "NotAnObject", limit: "500m"},
	}
	for _, tt := range invalid {
		t.Run("Invalid_"+tt.name, func(t *testing.T) {
			conf := modules.MustJSON(map[string]any{
				"replicas":      1,
				"env_variables": baseEnvVariables(),
				"limits":        tt.limit,
			})

			_, err := readConfig(res, conf, testDriverConf())
			assert.Error(t, err)
		})
	}

	t.Run("Invalid_MergedRequestsExceedDriverDefaultLimits", func(t *testing.T) {
		// driver default limits.cpu is 200m; requesting more than that
		// without also raising the limit must be rejected.
		conf := modules.MustJSON(map[string]any{
			"replicas":      1,
			"env_variables": baseEnvVariables(),
			"requests":      map[string]any{"cpu": "500m"},
		})

		_, err := readConfig(res, conf, testDriverConf())
		assert.Error(t, err)
	})

	t.Run("Invalid_MergedLimitsCPUIsZero", func(t *testing.T) {
		conf := modules.MustJSON(map[string]any{
			"replicas":      1,
			"env_variables": baseEnvVariables(),
			"limits":        map[string]any{"cpu": "0"},
		})

		_, err := readConfig(res, conf, testDriverConf())
		assert.Error(t, err)
	})
}

func Test_safeReleaseName(t *testing.T) {
	t.Parallel()

	table := []struct {
		str  string
		want string
	}{
		{
			str:  "abcd-efgh",
			want: "abcd-efgh-firehose",
		},
		{
			str:  "abcd-efgh-firehose",
			want: "abcd-efgh-firehose",
		},
		{
			str:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghijklmnopqrstuvwxyz",
			want: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghij-3801d0-firehose",
		},
		{
			str:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghi---klmnopqrstuvwxyz",
			want: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghi-81c192-firehose",
		},
		{
			str:  "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghijklmnopqr-stuvwxyz1234567890",
			want: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghij-bac696-firehose",
		},
	}

	for i, tt := range table {
		t.Run(fmt.Sprintf("Case#%d", i), func(t *testing.T) {
			got := modules.SafeName(tt.str, "-firehose", helmReleaseNameMaxLength)
			assert.Equal(t, tt.want, got)
			assert.True(t, len(got) <= helmReleaseNameMaxLength, "release name has length %d", len(got))
		})
	}
}
