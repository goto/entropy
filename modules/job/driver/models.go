package driver

import (
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/errors"
)

const (
	KeyKubeDependency = "kube_cluster"
	StopAction        = "stop"
)

const (
	Create PendingStep = iota
)

type (
	PendingStep   int
	TransientData struct {
		PendingSteps []PendingStep `json:"pending_steps"`
	}
)

func ReadTransientData(exr module.ExpandedResource) (*TransientData, error) {
	if len(exr.Resource.State.ModuleData) == 0 {
		return &TransientData{}, nil
	}

	var modData TransientData
	if err := json.Unmarshal(exr.Resource.State.ModuleData, &modData); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted transient data").WithCausef(err.Error())
	}
	return &modData, nil
}
