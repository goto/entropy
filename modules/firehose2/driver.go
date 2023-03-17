package firehose2

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
)

const (
	stepReleaseCreate = "release_create"
	stepReleaseUpdate = "release_update"
	stepReleaseStop   = "release_stop"
	stepConsumerReset = "consumer_reset"
)

type firehoseDriver struct {
	conf driverConf
}

func (fd *firehoseDriver) Plan(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	switch act.Name {
	case module.CreateAction:
		return fd.planCreate(ctx, exr, act)

	case module.UpdateAction:
		return fd.planUpdate(ctx, exr, act)

	case ResetAction:
		return fd.planReset(ctx, exr, act)

	default:
		return nil, errors.ErrInternal.
			WithMsgf("invalid action").
			WithCausef("action '%s' is not valid", act.Name)
	}
}

func (fd *firehoseDriver) Sync(ctx context.Context, exr module.ExpandedResource) (*resource.State, error) {
	output, err := readOutputData(exr)
	if err != nil {
		return nil, err
	}

	conf, err := readConfig(exr.Resource, exr.Spec.Configs)
	if err != nil {
		return nil, err
	}

	modData, err := readTransientData(exr)
	if err != nil {
		return nil, err
	}

	var kubeOut kubernetes.Output
	if err := json.Unmarshal(exr.Dependencies[keyKubeDependency].Output, &kubeOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}
}

func (fd *firehoseDriver) Output(ctx context.Context, res module.ExpandedResource) (json.RawMessage, error) {
	// TODO implement me
	panic("implement me")
}

func (fd *firehoseDriver) planCreate(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	conf, err := readConfig(exr.Resource, act.Params)
	if err != nil {
		return nil, err
	}

	exr.Resource.Spec.Configs = mustJSON(conf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: mustJSON(Output{
			Defaults: fd.conf,
		}),
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{stepReleaseCreate},
		}),
	}

	return &module.Plan{
		Reason:        "create_firehose",
		Resource:      exr.Resource,
		ScheduleRunAt: conf.StopTime,
	}, nil
}

func (fd *firehoseDriver) planUpdate(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	newConf, err := readConfig(exr.Resource, act.Params)
	if err != nil {
		return nil, err
	}

	exr.Resource.Spec.Configs = mustJSON(newConf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{stepReleaseUpdate},
		}),
	}
	return &module.Plan{
		Reason:        "update_firehose",
		Resource:      exr.Resource,
		ScheduleRunAt: newConf.StopTime,
	}, nil
}

func (fd *firehoseDriver) planReset(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	resetValue, err := prepResetValue(act.Params)
	if err != nil {
		return nil, err
	}

	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
		ModuleData: mustJSON(transientData{
			ResetTo: resetValue,
			PendingSteps: []string{
				stepReleaseStop,   // stop the deployment.
				stepConsumerReset, // reset the consumer group offset value.
				stepReleaseUpdate, // restart the deployment.
			},
		}),
	}
	return &module.Plan{
		Reason:   "reset_firehose",
		Resource: exr.Resource,
	}, nil
}

func prepResetValue(params json.RawMessage) (string, error) {
	var resetParams struct {
		To       string `json:"to"`
		Datetime string `json:"datetime"`
	}
	if err := json.Unmarshal(params, &resetParams); err != nil {
		return "", errors.ErrInvalid.
			WithMsgf("invalid params for reset action").
			WithCausef(err.Error())
	}

	resetValue := resetParams.To
	if resetParams.To == "DATETIME" {
		resetValue = resetParams.Datetime
	}
	return resetValue, nil
}
