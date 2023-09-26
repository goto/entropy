package driver

import (
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/job/config"
	"github.com/goto/entropy/modules/utils"
)

func (driver *Driver) planCreate(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	conf, err := config.ReadConfig(exr.Resource, act.Params, driver.Conf)
	if err != nil {
		return nil, err
	}
	conf.Namespace = driver.Conf.Namespace
	immediately := time.Now()
	exr.Resource.Spec.Configs = utils.MustJSON(conf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: utils.MustJSON(Output{
			Namespace: conf.Namespace,
			JobName:   conf.Name,
		}),
		NextSyncAt: &immediately,
		ModuleData: utils.MustJSON(TransientData{
			PendingSteps: []PendingStep{Create},
		}),
	}
	return &exr.Resource, nil
}
