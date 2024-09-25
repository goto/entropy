package dagger

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kafka"
)

const SourceKafkaConsumerAutoOffsetReset = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"
const (
	JobStateRunning   = "running"
	JobStateSuspended = "suspended"
	StateDeployed     = "DEPLOYED"
	StateUserStopped  = "USER_STOPPED"
)

func (dd *daggerDriver) Plan(_ context.Context, exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	switch act.Name {
	case module.CreateAction:
		return dd.planCreate(exr, act)

	case ResetAction:
		return dd.planReset(exr, act)

	case TriggerSavepointAction:
		return dd.planTriggerSavepoint(exr)

	default:
		return dd.planChange(exr, act)
	}
}

func (dd *daggerDriver) planCreate(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	conf, err := readConfig(exr, act.Params, dd.conf)
	if err != nil {
		return nil, err
	}

	//transformation #12
	conf.EnvVariables[keyStreams] = string(mustMarshalJSON(conf.Source))

	chartVals, err := mergeChartValues(&dd.conf.ChartValues, conf.ChartValues)
	if err != nil {
		return nil, err
	}
	conf.ChartValues = chartVals

	immediately := dd.timeNow()
	conf.JarURI = dd.conf.JarURI
	conf.State = StateDeployed
	conf.JobState = JobStateRunning
	conf.SavepointTriggerNonce = 1

	exr.Resource.Spec.Configs = modules.MustJSON(conf)

	err = dd.validateHelmReleaseConfigs(exr, *conf)
	if err != nil {
		return nil, err
	}

	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: modules.MustJSON(Output{
			Namespace: conf.Namespace,
		}),
		NextSyncAt: &immediately,
		ModuleData: modules.MustJSON(transientData{
			PendingSteps: []string{stepReleaseCreate},
		}),
	}

	return &exr.Resource, nil
}

func (dd *daggerDriver) planChange(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	curConf, err := readConfig(exr, exr.Resource.Spec.Configs, dd.conf)
	if err != nil {
		return nil, err
	}

	switch act.Name {
	case module.UpdateAction:
		newConf, err := readConfig(exr, act.Params, dd.conf)
		if err != nil {
			return nil, err
		}

		newConf.Source = mergeConsumerGroupId(curConf.Source, newConf.Source)

		chartVals, err := mergeChartValues(curConf.ChartValues, newConf.ChartValues)
		if err != nil {
			return nil, err
		}

		// restore configs that are not user-controlled.
		newConf.DeploymentID = curConf.DeploymentID
		newConf.ChartValues = chartVals
		newConf.JarURI = curConf.JarURI
		newConf.State = StateDeployed
		newConf.JobState = JobStateRunning

		newConf.Resources = mergeResources(curConf.Resources, newConf.Resources)

		curConf = newConf

	case StopAction:
		curConf.State = StateUserStopped
		curConf.JobState = JobStateSuspended

	case StartAction:
		curConf.State = StateDeployed
		curConf.JobState = JobStateRunning

	}
	immediately := dd.timeNow()

	exr.Resource.Spec.Configs = modules.MustJSON(curConf)

	err = dd.validateHelmReleaseConfigs(exr, *curConf)
	if err != nil {
		return nil, err
	}

	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
		ModuleData: modules.MustJSON(transientData{
			PendingSteps: []string{stepReleaseUpdate},
		}),
		NextSyncAt: &immediately,
	}

	return &exr.Resource, nil
}

func (dd *daggerDriver) planReset(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	resetValue, err := kafka.ParseResetV2Params(act.Params)
	if err != nil {
		return nil, err
	}

	immediately := dd.timeNow()

	curConf, err := readConfig(exr, exr.Resource.Spec.Configs, dd.conf)
	if err != nil {
		return nil, err
	}

	curConf.ResetOffset = resetValue

	curConf.Source = dd.consumerReset(context.Background(), *curConf, resetValue)
	curConf.EnvVariables[keyStreams] = string(mustMarshalJSON(curConf.Source))

	exr.Resource.Spec.Configs = modules.MustJSON(curConf)
	exr.Resource.State = resource.State{
		Status:     resource.StatusPending,
		Output:     exr.Resource.State.Output,
		NextSyncAt: &immediately,
		ModuleData: modules.MustJSON(transientData{
			ResetOffsetTo: resetValue,
			PendingSteps: []string{
				stepKafkaReset,
			},
		}),
	}
	return &exr.Resource, nil
}

func (dd *daggerDriver) planTriggerSavepoint(exr module.ExpandedResource) (*resource.Resource, error) {
	curConf, err := readConfig(exr, exr.Resource.Spec.Configs, dd.conf)
	if err != nil {
		return nil, err
	}

	curConf.SavepointTriggerNonce += 1

	immediately := dd.timeNow()

	exr.Resource.Spec.Configs = modules.MustJSON(curConf)

	err = dd.validateHelmReleaseConfigs(exr, *curConf)
	if err != nil {
		return nil, err
	}

	exr.Resource.State = resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
		ModuleData: modules.MustJSON(transientData{
			PendingSteps: []string{stepSavepointCreate},
		}),
		NextSyncAt: &immediately,
	}

	return &exr.Resource, nil
}

func (dd *daggerDriver) validateHelmReleaseConfigs(expandedResource module.ExpandedResource, config Config) error {
	var flinkOut flink.Output
	if err := json.Unmarshal(expandedResource.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}

	_, err := dd.getHelmRelease(expandedResource.Resource, config, flinkOut.KubeCluster)
	return err
}

func mergeConsumerGroupId(currStreams, newStreams []Source) []Source {
	if len(currStreams) != len(newStreams) {
		return newStreams
	}

	for i := range currStreams {
		if currStreams[i].SourceParquet.SourceParquetFilePaths != nil && len(currStreams[i].SourceParquet.SourceParquetFilePaths) > 0 {
			//source is parquete
			//do nothing
			continue
		}

		if currStreams[i].SourceKafka.SourceKafkaName == newStreams[i].SourceKafka.SourceKafkaName {
			newStreams[i].SourceKafka.SourceKafkaConsumerConfigGroupID = currStreams[i].SourceKafka.SourceKafkaConsumerConfigGroupID
		}
	}

	return newStreams
}
