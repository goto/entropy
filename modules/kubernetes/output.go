package kubernetes

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/version"

	"github.com/goto/entropy/pkg/kube"
)

type Output struct {
	Configs        kube.Config                             `json:"configs"`
	ServerInfo     version.Info                            `json:"server_info"`
	TolerationMode string                                  `json:"toleration_mode"`
	Tolerations    map[string][]Toleration                 `json:"tolerations"`
	AffinityMode   string                                  `json:"affinity_mode"`
	Affinities     map[string]NodeAffinityMatchExpressions `json:"affinities"`
}

type Toleration struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Effect   string `json:"effect"`
	Operator string `json:"operator"`
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

func (out Output) JSON() []byte {
	b, err := json.Marshal(out)
	if err != nil {
		panic(err)
	}
	return b
}
