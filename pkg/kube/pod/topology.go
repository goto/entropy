package pod

// TopologySpreadConstraint is the workload independent part of a pod topology
// spread constraint, set once per project in the module config.
//
// The label selector is not part of it on purpose: it must match the pod labels
// of every single release, which only the helm chart knows. The chart fills in
// labelSelector (and matchLabelKeys) for each constraint.
type TopologySpreadConstraint struct {
	MaxSkew           int    `json:"max_skew" validate:"required,min=1"`
	TopologyKey       string `json:"topology_key" validate:"required"`
	WhenUnsatisfiable string `json:"when_unsatisfiable" validate:"required,oneof=DoNotSchedule ScheduleAnyway"`

	// NodeTaintsPolicy defaults to Honor in the chart, so tainted (cordoned or
	// failed) nodes do not count towards the skew.
	NodeTaintsPolicy string `json:"node_taints_policy,omitempty" validate:"omitempty,oneof=Honor Ignore"`

	// MinDomains is only needed when a nodepool can scale down to zero nodes.
	MinDomains int `json:"min_domains,omitempty" validate:"omitempty,min=1"`
}

// TopologySpreadConstraintsToValues converts the constraints to helm chart
// values. It returns nil for an empty list, so callers can skip the chart value
// and keep the rendered manifest unchanged.
func TopologySpreadConstraintsToValues(constraints []TopologySpreadConstraint) []map[string]any {
	if len(constraints) == 0 {
		return nil
	}

	values := make([]map[string]any, 0, len(constraints))
	for _, c := range constraints {
		v := map[string]any{
			"maxSkew":           c.MaxSkew,
			"topologyKey":       c.TopologyKey,
			"whenUnsatisfiable": c.WhenUnsatisfiable,
		}
		if c.NodeTaintsPolicy != "" {
			v["nodeTaintsPolicy"] = c.NodeTaintsPolicy
		}
		if c.MinDomains > 0 {
			v["minDomains"] = c.MinDomains
		}
		values = append(values, v)
	}
	return values
}
