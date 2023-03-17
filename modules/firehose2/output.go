package firehose2

import (
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube"
)

type Output struct {
	Pods        []kube.Pod `json:"pods,omitempty"`
	Defaults    driverConf `json:"defaults,omitempty"`
	Namespace   string     `json:"namespace,omitempty"`
	ReleaseName string     `json:"release_name,omitempty"`
}

type transientData struct {
	ResetTo       string   `json:"reset_to,omitempty"`
	PendingSteps  []string `json:"pending_steps"`
	StateOverride string   `json:"state_override,omitempty"`
}

func readOutputData(exr module.ExpandedResource) (*Output, error) {
	var curOut Output
	if err := json.Unmarshal(exr.Resource.State.Output, &curOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted output").WithCausef(err.Error())
	}
	return &curOut, nil
}

func readTransientData(exr module.ExpandedResource) (*transientData, error) {
	var modData transientData
	if err := json.Unmarshal(exr.Resource.State.Output, &modData); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted transient data").WithCausef(err.Error())
	}
	return &modData, nil
}
