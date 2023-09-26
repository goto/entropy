package driver

import (
	"context"
	"encoding/json"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/job/config"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/modules/utils"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube"
	"github.com/goto/entropy/pkg/kube/job"
)

type Driver struct {
	Conf      config.DriverConf
	CreateJob func(ctx context.Context, conf kube.Config, j *job.Job) error
}

func (driver *Driver) Plan(_ context.Context, res module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	switch act.Name {
	case module.CreateAction:
		return driver.planCreate(res, act)
	default:
		return &resource.Resource{}, nil
	}
}

func (driver *Driver) Sync(ctx context.Context, exr module.ExpandedResource) (*resource.State, error) {
	modData, err := ReadTransientData(exr)
	if err != nil {
		return nil, err
	}

	out, err := ReadOutputData(exr)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	conf, err := config.ReadConfig(exr.Resource, exr.Spec.Configs, driver.Conf)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	var kubeOut kubernetes.Output
	if err := json.Unmarshal(exr.Dependencies[KeyKubeDependency].Output, &kubeOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}

	finalState := resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
	}

	if len(modData.PendingSteps) > 0 {
		pendingStep := modData.PendingSteps[0]
		modData.PendingSteps = modData.PendingSteps[1:]
		switch pendingStep {
		case Create:
			if err := driver.create(ctx, exr.Resource, conf, kubeOut); err != nil {
				return nil, err
			}
		default:
			return nil, errors.ErrInternal.WithMsgf("unknown step:")
		}

		immediately := time.Now()
		finalState.NextSyncAt = &immediately
		finalState.ModuleData = utils.MustJSON(modData)

		return &finalState, nil
	}

	finalOut, err := driver.refreshOutput(ctx, exr.Resource, *conf, *out, kubeOut)
	if err != nil {
		return nil, err
	}
	finalState.Output = finalOut

	finalState.Status = resource.StatusCompleted
	finalState.ModuleData = nil
	return &finalState, nil
}

func (*Driver) Output(context.Context, module.ExpandedResource) (json.RawMessage, error) {
	return nil, nil
}
