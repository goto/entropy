package driver

import (
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/job/config"
)

const (
	KeyKubeDependency = "kube_cluster"
	StopAction        = "stop"
)

const (
	Create = "create"
)

type (
	PendingStep   string
	TransientData struct {
		PendingSteps []PendingStep `json:"pending_steps"`
	}
)

func (driver *Driver) planCreate(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	conf, err := config.ReadConfig(exr.Resource, act.Params, driver.Conf)
	if err != nil {
		return nil, err
	}
	conf.Namespace = driver.Conf.Namespace
	immediately := time.Now()
	exr.Resource.Spec.Configs = modules.MustJSON(conf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: modules.MustJSON(Output{
			Namespace: conf.Namespace,
			JobName:   conf.Name,
		}),
		NextSyncAt: &immediately,
		ModuleData: modules.MustJSON(TransientData{
			PendingSteps: []PendingStep{Create},
		}),
	}
	return &exr.Resource, nil
}
