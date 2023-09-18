package firehose

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kafka"
)

const SourceKafkaConsumerAutoOffsetReset = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"

var errGroupNumberLimitCrossed = errors.New("group number limit crossed 9999")

func (fd *firehoseDriver) Plan(_ context.Context, exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	switch act.Name {
	case module.CreateAction:
		return fd.planCreate(exr, act)

	case ResetAction:
		return fd.planReset(exr, act)

	case NewResetAction:
		return fd.planNewReset(exr, act)

	default:
		return fd.planChange(exr, act)
	}
}

func (fd *firehoseDriver) planChange(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	curConf, err := readConfig(exr.Resource, exr.Resource.Spec.Configs, fd.conf)
	if err != nil {
		return nil, err
	}

	switch act.Name {
	case module.UpdateAction:
		newConf, err := readConfig(exr.Resource, act.Params, fd.conf)
		if err != nil {
			return nil, err
		}

		chartVals, err := mergeChartValues(curConf.ChartValues, newConf.ChartValues)
		if err != nil {
			return nil, err
		}

		// restore configs that are not user-controlled.
		newConf.DeploymentID = curConf.DeploymentID
		newConf.ChartValues = chartVals
		newConf.Namespace = curConf.Namespace
		newConf.Telegraf = fd.conf.Telegraf
		newConf.InitContainer = fd.conf.InitContainer

		curConf = newConf

	case ScaleAction:
		var scaleParams ScaleParams
		if err := json.Unmarshal(act.Params, &scaleParams); err != nil {
			return nil, errors.ErrInvalid.WithMsgf("invalid params for scale action").WithCausef(err.Error())
		} else if scaleParams.Replicas < 1 {
			return nil, errors.ErrInvalid.WithMsgf("replicas must be >= 1")
		}

		curConf.Replicas = scaleParams.Replicas

	case StartAction:
		var startParams StartParams
		if err := json.Unmarshal(act.Params, &startParams); err != nil {
			return nil, errors.ErrInvalid.WithMsgf("invalid params for start action").WithCausef(err.Error())
		}
		curConf.Stopped = false
		if startParams.StopTime != nil {
			curConf.StopTime = startParams.StopTime
		}

	case StopAction:
		curConf.Stopped = true

	case UpgradeAction:
		// upgrade the chart values to the latest project-level config.
		// Note: upgrade/downgrade will happen based on module-level configs.
		curConf.ChartValues = &fd.conf.ChartValues
	}

	immediately := fd.timeNow()

	exr.Resource.Spec.Configs = mustJSON(curConf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{stepReleaseUpdate},
		}),
		NextSyncAt: &immediately,
	}

	return &exr.Resource, nil
}

func (fd *firehoseDriver) planCreate(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	conf, err := readConfig(exr.Resource, act.Params, fd.conf)
	if err != nil {
		return nil, err
	}

	chartVals, err := mergeChartValues(&fd.conf.ChartValues, conf.ChartValues)
	if err != nil {
		return nil, err
	}

	// set project defaults.
	conf.Telegraf = fd.conf.Telegraf
	conf.Namespace = fd.conf.Namespace
	conf.ChartValues = chartVals

	immediately := fd.timeNow()

	exr.Resource.Spec.Configs = mustJSON(conf)
	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: mustJSON(Output{
			Namespace:   conf.Namespace,
			ReleaseName: conf.DeploymentID,
		}),
		NextSyncAt: &immediately,
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{stepReleaseCreate},
		}),
	}

	return &exr.Resource, nil
}

func (fd *firehoseDriver) planNewReset(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	resetValue, err := kafka.ParseNewResetParams(act.Params)
	if err != nil {
		return nil, err
	}

	immediately := fd.timeNow()

	curConf, err := readConfig(exr.Resource, exr.Resource.Spec.Configs, fd.conf)
	if err != nil {
		return nil, err
	}

	curConf.ResetOffset = resetValue

	exr.Resource.Spec.Configs = mustJSON(curConf)
	exr.Resource.State = resource.State{
		Status:     resource.StatusPending,
		Output:     exr.Resource.State.Output,
		NextSyncAt: &immediately,
		ModuleData: mustJSON(transientData{
			ResetOffsetTo: resetValue,
			PendingSteps: []string{
				stepReleaseStop,   // stop the firehose
				stepKafkaReset,    // reset the consumer group offset value.
				stepReleaseUpdate, // restart the deployment.
			},
		}),
	}
	return &exr.Resource, nil
}

func (fd *firehoseDriver) planReset(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	resetValue, err := kafka.ParseResetParams(act.Params)
	if err != nil {
		return nil, err
	}

	immediately := fd.timeNow()

	curConf, err := readConfig(exr.Resource, exr.Resource.Spec.Configs, fd.conf)
	if err != nil {
		return nil, err
	}

	curConf.ResetOffset = resetValue
	curConf.EnvVariables[SourceKafkaConsumerAutoOffsetReset] = resetValue
	curConf.EnvVariables[confKeyConsumerID], err = getNewConsumerGroupID(curConf.EnvVariables[confKeyConsumerID], curConf.DeploymentID)
	if err != nil {
		return nil, err
	}

	exr.Resource.Spec.Configs = mustJSON(curConf)
	exr.Resource.State = resource.State{
		Status:     resource.StatusPending,
		Output:     exr.Resource.State.Output,
		NextSyncAt: &immediately,
		ModuleData: mustJSON(transientData{
			PendingSteps: []string{
				stepReleaseStop,   // stop the firehose
				stepReleaseUpdate, // restart the deployment.
			},
		}),
	}
	return &exr.Resource, nil
}

func getNewConsumerGroupID(currentConsumerGroupID, deploymentID string) (string, error) {
	const groupNumberSuffixLength = 4
	suffix := "0000"

	currentConsumerGroupSuffix := currentConsumerGroupID[len(currentConsumerGroupID)-groupNumberSuffixLength:]
	currentConsumerGroupNumber, err := strconv.Atoi(currentConsumerGroupSuffix)
	if err != nil {
		return "", err
	}

	currentConsumerGroupNumber++
	newConsumerGroupSuffix := strconv.Itoa(currentConsumerGroupNumber)

	if len(newConsumerGroupSuffix) > groupNumberSuffixLength {
		return "", errGroupNumberLimitCrossed
	}

	newConsumerGroupSuffix = suffix[:groupNumberSuffixLength-len(newConsumerGroupSuffix)] + newConsumerGroupSuffix

	return fmt.Sprintf("%s-%s", deploymentID, newConsumerGroupSuffix), nil
}
