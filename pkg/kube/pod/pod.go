package pod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/goto/entropy/pkg/kube/container"
	"github.com/goto/entropy/pkg/kube/volume"
)

type Toleration struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Effect   string `json:"effect"`
	Operator string `json:"operator"`
}

func (t *Toleration) ToCoreV1() *corev1.Toleration {
	if t == nil {
		return nil
	}
	return &corev1.Toleration{
		Key:      t.Key,
		Value:    t.Value,
		Effect:   corev1.TaintEffect(t.Effect),
		Operator: corev1.TolerationOperator(t.Operator),
	}
}

type NodeAffinityMatchExpressions struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []Preference         `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPreference `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type WeightedPreference struct {
	Weight     int          `json:"weight" validate:"required"`
	Preference []Preference `json:"preference" validate:"required"`
}

type Preference struct {
	Key      string   `json:"key" validate:"required"`
	Operator string   `json:"operator" validate:"required"`
	Values   []string `json:"values"`
}

func (na *NodeAffinityMatchExpressions) ToCoreV1() *corev1.NodeAffinity {
	if na == nil {
		return nil
	}

	nodeSelectorsTerm := []corev1.NodeSelectorTerm{}
	if len(na.RequiredDuringSchedulingIgnoredDuringExecution) > 0 {
		nodeSelectorsTerm = append(nodeSelectorsTerm, corev1.NodeSelectorTerm{
			MatchExpressions: func() []corev1.NodeSelectorRequirement {
				var reqs []corev1.NodeSelectorRequirement
				for _, expr := range na.RequiredDuringSchedulingIgnoredDuringExecution {
					reqs = append(reqs, corev1.NodeSelectorRequirement{
						Key:      expr.Key,
						Operator: corev1.NodeSelectorOperator(expr.Operator),
						Values:   expr.Values,
					})
				}
				return reqs
			}(),
		})
	}

	preferredSchedulingTerm := []corev1.PreferredSchedulingTerm{}
	if len(na.PreferredDuringSchedulingIgnoredDuringExecution) > 0 {
		for _, wp := range na.PreferredDuringSchedulingIgnoredDuringExecution {
			preferredSchedulingTerm = append(preferredSchedulingTerm, corev1.PreferredSchedulingTerm{
				Weight: int32(wp.Weight),
				Preference: corev1.NodeSelectorTerm{
					MatchExpressions: func() []corev1.NodeSelectorRequirement {
						var reqs []corev1.NodeSelectorRequirement
						for _, expr := range wp.Preference {
							reqs = append(reqs, corev1.NodeSelectorRequirement{
								Key:      expr.Key,
								Operator: corev1.NodeSelectorOperator(expr.Operator),
								Values:   expr.Values,
							})
						}
						return reqs
					}(),
				},
			})
		}
	}

	return &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
			NodeSelectorTerms: nodeSelectorsTerm,
		},
		PreferredDuringSchedulingIgnoredDuringExecution: preferredSchedulingTerm,
	}
}

type Pod struct {
	Name       string
	Containers []container.Container
	Volumes    []volume.Volume
	Labels     map[string]string
	// Tolerations represents the tolerations to be set for the deployment.
	// The key in the map is the sink-type in upper case.
	Tolerations []Toleration
	// NodeAffinityMatchExpressions can be used to set node-affinity for the deployment.
	NodeAffinityMatchExpressions *NodeAffinityMatchExpressions
}

func (p Pod) Template() corev1.PodTemplateSpec {
	var tolerations []corev1.Toleration

	if len(p.Tolerations) > 0 {
		for _, t := range p.Tolerations {
			tolerations = append(tolerations, *t.ToCoreV1())
		}
	}

	var affinity *corev1.Affinity

	if p.NodeAffinityMatchExpressions != nil {
		affinity = &corev1.Affinity{NodeAffinity: p.NodeAffinityMatchExpressions.ToCoreV1()}
	}

	var containers []corev1.Container
	for _, c := range p.Containers {
		containers = append(containers, c.Template())
	}
	var volumes []corev1.Volume
	for _, v := range p.Volumes {
		volumes = append(volumes, v.GetPodVolume())
	}
	volumes = append(volumes, corev1.Volume{
		Name:         "shared-data",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	})
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.Name,
			Labels: p.Labels,
		},
		Spec: corev1.PodSpec{
			Tolerations:   tolerations,
			Affinity:      affinity,
			Containers:    containers,
			Volumes:       volumes,
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}
