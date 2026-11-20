package dagger

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/kube/pod"
	"github.com/goto/entropy/pkg/validator"
)

func topologyTestRelease(t *testing.T, tsc []pod.TopologySpreadConstraint) map[string]any {
	t.Helper()

	dd := &daggerDriver{conf: driverConf{TopologySpreadConstraints: tsc}}
	rc, err := dd.getHelmRelease(resource.Resource{
		URN:     "orn:entropy:dagger:test:tsc-test",
		Name:    "tsc-test",
		Project: "test-project",
	}, Config{
		ChartValues:  &ChartValues{},
		EnvVariables: map[string]string{},
	}, kubernetes.Output{})
	require.NoError(t, err)
	return rc.Values
}

func TestGetHelmReleaseTopologySpreadConstraints(t *testing.T) {
	t.Run("not configured leaves the chart value out", func(t *testing.T) {
		values := topologyTestRelease(t, nil)
		assert.NotContains(t, values, "topologySpreadConstraints")
	})

	t.Run("configured constraints are passed to the chart", func(t *testing.T) {
		values := topologyTestRelease(t, []pod.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: "DoNotSchedule"},
			{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: "ScheduleAnyway"},
		})

		assert.Equal(t, []map[string]any{
			{"maxSkew": 1, "topologyKey": "topology.kubernetes.io/zone", "whenUnsatisfiable": "DoNotSchedule"},
			{"maxSkew": 1, "topologyKey": "kubernetes.io/hostname", "whenUnsatisfiable": "ScheduleAnyway"},
		}, values["topologySpreadConstraints"])
	})

	t.Run("does not change the other chart values", func(t *testing.T) {
		without := topologyTestRelease(t, nil)
		with := topologyTestRelease(t, []pod.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: "DoNotSchedule"},
		})

		delete(with, "topologySpreadConstraints")
		assert.Equal(t, without, with)
	})
}

func TestDriverConfTopologySpreadConstraints(t *testing.T) {
	const base = `{
		"namespace": {"default": "dagger"},
		"chart_values": {"chart_version": "0.9.5", "image_repository": "gotocompany/dagger", "image_tag": "0.1.0"},
		"resources": {"taskmanager": {"cpu": "1", "memory": "1Gi"}, "jobmanager": {"cpu": "1", "memory": "1Gi"}},
		"jar_uri": "local:///opt/flink/lib/dagger.jar"%s
	}`

	tests := []struct {
		name    string
		extra   string
		wantErr bool
		wantTSC []pod.TopologySpreadConstraint
	}{
		{
			name: "no constraints",
		},
		{
			name: "valid constraints",
			extra: `, "topology_spread_constraints": [
				{"max_skew": 1, "topology_key": "topology.kubernetes.io/zone", "when_unsatisfiable": "DoNotSchedule"},
				{"max_skew": 1, "topology_key": "kubernetes.io/hostname", "when_unsatisfiable": "ScheduleAnyway"}
			]`,
			wantTSC: []pod.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: "DoNotSchedule"},
				{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: "ScheduleAnyway"},
			},
		},
		{
			name:    "invalid when_unsatisfiable is rejected",
			extra:   `, "topology_spread_constraints": [{"max_skew": 1, "topology_key": "topology.kubernetes.io/zone", "when_unsatisfiable": "Never"}]`,
			wantErr: true,
		},
		{
			name:    "missing topology_key is rejected",
			extra:   `, "topology_spread_constraints": [{"max_skew": 1, "when_unsatisfiable": "DoNotSchedule"}]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Same steps as Module().DriverFactory, but into a fresh config.
			// The factory copies defaultDriverConf shallowly, so decoding
			// through it would leak values into other tests.
			var conf driverConf
			require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(base, tt.extra)), &conf))
			err := validator.TaggedStruct(conf)
			if tt.wantErr {
				assert.ErrorContains(t, err, "TopologySpreadConstraints")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantTSC, conf.TopologySpreadConstraints)
		})
	}
}
