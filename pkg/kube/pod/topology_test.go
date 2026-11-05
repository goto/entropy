package pod

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/goto/entropy/pkg/validator"
)

func TestTopologySpreadConstraintsToValues(t *testing.T) {
	t.Run("empty list returns nil", func(t *testing.T) {
		assert.Nil(t, TopologySpreadConstraintsToValues(nil))
		assert.Nil(t, TopologySpreadConstraintsToValues([]TopologySpreadConstraint{}))
	})

	t.Run("zone and host constraints", func(t *testing.T) {
		got := TopologySpreadConstraintsToValues([]TopologySpreadConstraint{
			{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: "DoNotSchedule"},
			{MaxSkew: 1, TopologyKey: "kubernetes.io/hostname", WhenUnsatisfiable: "ScheduleAnyway", NodeTaintsPolicy: "Ignore", MinDomains: 3},
		})

		assert.Equal(t, []map[string]any{
			{"maxSkew": 1, "topologyKey": "topology.kubernetes.io/zone", "whenUnsatisfiable": "DoNotSchedule"},
			{"maxSkew": 1, "topologyKey": "kubernetes.io/hostname", "whenUnsatisfiable": "ScheduleAnyway", "nodeTaintsPolicy": "Ignore", "minDomains": 3},
		}, got)
	})
}

func TestTopologySpreadConstraintValidation(t *testing.T) {
	valid := TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "topology.kubernetes.io/zone", WhenUnsatisfiable: "DoNotSchedule"}

	tests := []struct {
		name    string
		mutate  func(c *TopologySpreadConstraint)
		wantErr bool
	}{
		{name: "valid", mutate: func(*TopologySpreadConstraint) {}},
		{name: "valid with optional fields", mutate: func(c *TopologySpreadConstraint) { c.NodeTaintsPolicy = "Honor"; c.MinDomains = 3 }},
		{name: "max_skew zero", mutate: func(c *TopologySpreadConstraint) { c.MaxSkew = 0 }, wantErr: true},
		{name: "missing topology_key", mutate: func(c *TopologySpreadConstraint) { c.TopologyKey = "" }, wantErr: true},
		{name: "bad when_unsatisfiable", mutate: func(c *TopologySpreadConstraint) { c.WhenUnsatisfiable = "Never" }, wantErr: true},
		{name: "bad node_taints_policy", mutate: func(c *TopologySpreadConstraint) { c.NodeTaintsPolicy = "Always" }, wantErr: true},
		{name: "negative min_domains", mutate: func(c *TopologySpreadConstraint) { c.MinDomains = -1 }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid
			tt.mutate(&c)
			err := validator.TaggedStruct(c)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
