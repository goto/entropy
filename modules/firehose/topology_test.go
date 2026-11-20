package firehose

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/kube/pod"
	"github.com/goto/entropy/pkg/validator"
)

func topologyTestRelease(t *testing.T, tsc []pod.TopologySpreadConstraint) map[string]any {
	t.Helper()

	res := resource.Resource{
		URN:     "orn:entropy:firehose:project-1:resource-1-firehose",
		Kind:    "firehose",
		Name:    "resource-1",
		Project: "project-1",
		Spec: resource.Spec{
			Configs: []byte(`{
				"env_variables": {
					"SINK_TYPE": "LOG",
					"INPUT_SCHEMA_PROTO_CLASS": "com.foo.Bar",
					"SOURCE_KAFKA_CONSUMER_GROUP_ID": "foo-bar-baz",
					"SOURCE_KAFKA_BROKERS": "localhost:9092",
					"SOURCE_KAFKA_TOPIC": "foo-log"
				},
				"replicas": 3
			}`),
		},
	}

	dc := firehoseDriverConf()
	dc.TopologySpreadConstraints = tsc
	fd := &firehoseDriver{conf: dc, timeNow: func() time.Time { return frozenTime }}

	conf, err := readConfig(res, res.Spec.Configs, fd.conf)
	require.NoError(t, err)
	conf.ChartValues, err = mergeChartValues(&fd.conf.ChartValues, conf.ChartValues)
	require.NoError(t, err)
	conf.Telegraf = fd.conf.Telegraf

	rc, err := fd.getHelmRelease(res, *conf, kubernetes.Output{})
	require.NoError(t, err)
	return rc.Values
}

func TestFirehoseGetHelmReleaseTopologySpreadConstraints(t *testing.T) {
	t.Run("not configured leaves the chart value out", func(t *testing.T) {
		values := topologyTestRelease(t, nil)
		assert.NotContains(t, values, "topologySpreadConstraints")
	})

	t.Run("configured constraints are passed to the chart", func(t *testing.T) {
		values := topologyTestRelease(t, []pod.TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: "DoNotSchedule"},
			{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: "ScheduleAnyway", NodeTaintsPolicy: "Honor"},
		})

		assert.Equal(t, []map[string]any{
			{"maxSkew": 1, "topologyKey": "topology.kubernetes.io/zone", "whenUnsatisfiable": "DoNotSchedule"},
			{"maxSkew": 1, "topologyKey": "kubernetes.io/hostname", "whenUnsatisfiable": "ScheduleAnyway", "nodeTaintsPolicy": "Honor"},
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

func TestFirehoseDriverConfTopologySpreadConstraints(t *testing.T) {
	raw, err := os.ReadFile("./test/module-config.json")
	require.NoError(t, err)

	withTSC := func(t *testing.T, tsc any) json.RawMessage {
		t.Helper()
		var conf map[string]any
		require.NoError(t, json.Unmarshal(raw, &conf))
		if tsc != nil {
			conf["topology_spread_constraints"] = tsc
		}
		b, err := json.Marshal(conf)
		require.NoError(t, err)
		return b
	}

	tests := []struct {
		name    string
		tsc     any
		wantErr bool
		wantTSC []pod.TopologySpreadConstraint
	}{
		{
			name: "no constraints",
		},
		{
			name: "valid constraints",
			tsc: []map[string]any{
				{"max_skew": 1, "topology_key": "topology.kubernetes.io/zone", "when_unsatisfiable": "DoNotSchedule"},
				{"max_skew": 1, "topology_key": "kubernetes.io/hostname", "when_unsatisfiable": "ScheduleAnyway"},
			},
			wantTSC: []pod.TopologySpreadConstraint{
				{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: "DoNotSchedule"},
				{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: "ScheduleAnyway"},
			},
		},
		{
			name:    "max_skew zero is rejected",
			tsc:     []map[string]any{{"max_skew": 0, "topology_key": "topology.kubernetes.io/zone", "when_unsatisfiable": "DoNotSchedule"}},
			wantErr: true,
		},
		{
			name:    "invalid node_taints_policy is rejected",
			tsc:     []map[string]any{{"max_skew": 1, "topology_key": "topology.kubernetes.io/zone", "when_unsatisfiable": "DoNotSchedule", "node_taints_policy": "Always"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Same steps as Module().DriverFactory, but into a fresh config.
			// The factory copies defaultDriverConf shallowly, so decoding a
			// fixture through it would leak values into other tests.
			conf := driverConf{ChartValues: defaultDriverConf.ChartValues}
			require.NoError(t, json.Unmarshal(withTSC(t, tt.tsc), &conf))
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
