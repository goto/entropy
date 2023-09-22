package driver

import (
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/errors"
)

type PendingStep int

const (
	KeyKubeDependency = "kube_cluster"
	StopAction        = "stop"
)

const (
	Create PendingStep = iota
)

type TransientData struct {
	PendingSteps []PendingStep `json:"pending_steps"`
}

type Output struct {
	Namespace string `json:"namespace"`
	JobName   string `json:"job_name"`
}

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

func ReadOutputData(exr module.ExpandedResource) (*Output, error) {
	var curOut Output
	if err := json.Unmarshal(exr.Resource.State.Output, &curOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted output").WithCausef(err.Error())
	}
	return &curOut, nil
}
