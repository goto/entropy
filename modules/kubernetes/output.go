package kubernetes

import (
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/version"

	"github.com/goto/entropy/pkg/kube"
	"github.com/mitchellh/mapstructure"
)

type Output struct {
	Configs        kube.Config                             `json:"configs"`
	ServerInfo     version.Info                            `json:"server_info"`
	TolerationMode map[string]string                       `json:"toleration_mode"`
	Tolerations    map[string][]Toleration                 `json:"tolerations"`
	AffinityMode   map[string]string                       `json:"affinity_mode"`
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

func PreferenceSliceToInterfaceSlice(prefs []Preference) []map[string]interface{} {
	result := make([]map[string]interface{}, len(prefs))

	for i, pref := range prefs {
		var prefMap map[string]interface{}
		if err := mapstructure.Decode(pref, &prefMap); err != nil {
			continue
		}

		lowercaseMap := make(map[string]interface{})
		for k, v := range prefMap {
			lowercaseMap[strings.ToLower(k)] = v
		}
		result[i] = lowercaseMap
	}

	return result
}

func WeightedPreferencesToInterfaceSlice(weightedPrefs []WeightedPreference) []map[string]interface{} {
	result := make([]map[string]interface{}, len(weightedPrefs))

	for i, wp := range weightedPrefs {
		var wpMap map[string]interface{}
		if err := mapstructure.Decode(wp, &wpMap); err != nil {
			continue
		}

		lowercaseMap := make(map[string]interface{})
		for k, v := range wpMap {
			// Special handling for the preference field
			if k == "Preference" && v != nil {
				// Convert the nested Preference slice
				lowercaseMap["preference"] = PreferenceSliceToInterfaceSlice(wp.Preference)
			} else {
				lowercaseMap[strings.ToLower(k)] = v
			}
		}
		result[i] = lowercaseMap
	}

	return result
}
