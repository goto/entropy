package firehose2

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

func (fd *firehoseDriver) Plan(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	switch act.Name {
	case module.CreateAction:
		return fd.planCreate(ctx, exr, act)

	case module.UpdateAction:
		return fd.planUpdate(ctx, exr, act)

	case ResetAction:
		return fd.planReset(ctx, exr, act)

	case UpgradeAction:
		return fd.planUpgrade(ctx, exr, act)

	default:
		return nil, errors.ErrInternal.
			WithMsgf("invalid action").
			WithCausef("action '%s' is not valid", act.Name)
	}
}

func (fd *firehoseDriver) planCreate(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	conf, err := readConfig(exr.Resource, act.Params)
	if err != nil {
		return nil, err
	}

	// set project defaults.
	conf.Telegraf = fd.conf.Telegraf
	conf.Namespace = fd.conf.Namespace
	conf.ChartValues = &fd.conf.ChartValues

	exr.Resource.Spec.Configs = mustJSON(conf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: mustJSON(Output{
			Namespace:   conf.Namespace,
			ReleaseName: conf.DeploymentID,
		}),
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{stepReleaseCreate},
		}),
	}

	return &module.Plan{
		Reason:   "create_firehose",
		Resource: exr.Resource,
	}, nil
}

func (fd *firehoseDriver) planUpdate(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	curConf, err := readConfig(exr.Resource, exr.Resource.Spec.Configs)
	if err != nil {
		return nil, err
	}

	newConf, err := readConfig(exr.Resource, act.Params)
	if err != nil {
		return nil, err
	}

	// restore configs that are not user-controlled.
	newConf.DeploymentID = curConf.DeploymentID
	newConf.ChartValues = curConf.ChartValues
	newConf.Namespace = curConf.Namespace
	newConf.Telegraf = curConf.Telegraf

	exr.Resource.Spec.Configs = mustJSON(newConf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{stepReleaseUpdate},
		}),
	}
	return &module.Plan{
		Reason:   "update_firehose",
		Resource: exr.Resource,
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
			ResetOffsetTo: resetValue,
			PendingSteps: []string{
				stepReleaseStop,   // stop the deployment.
				stepKafkaReset,    // reset the consumer group offset value.
				stepReleaseUpdate, // restart the deployment.
			},
		}),
	}
	return &module.Plan{
		Reason:   "reset_firehose",
		Resource: exr.Resource,
	}, nil
}

func (fd *firehoseDriver) planUpgrade(ctx context.Context, exr module.ExpandedResource, act module.ActionRequest) (*module.Plan, error) {
	curConf, err := readConfig(exr.Resource, exr.Resource.Spec.Configs)
	if err != nil {
		return nil, err
	}

	// upgrade the chart values to the latest project-level config.
	curConf.ChartValues = &fd.conf.ChartValues

	exr.Resource.Spec.Configs = mustJSON(curConf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{stepReleaseUpdate},
		}),
	}
	return &module.Plan{
		Reason:   "upgrade_firehose",
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

	resetValue := strings.ToLower(resetParams.To)
	if resetParams.To == "datetime" {
		resetValue = resetParams.Datetime
	} else if resetValue != "latest" && resetValue != "earliest" {
		return "", errors.ErrInvalid.
			WithMsgf("reset_value must be one of latest, earliest, datetime").
			WithCausef("'%s' is not valid reset value", resetValue)
	}
	return resetValue, nil
}
