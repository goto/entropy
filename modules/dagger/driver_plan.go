package dagger

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kafka"
)

const SourceKafkaConsumerAutoOffsetReset = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"
const (
	JobStateRunning                          = "running"
	JobStateSuspended                        = "suspended"
	StateDeployed                            = "DEPLOYED"
	StateUserStopped                         = "USER_STOPPED"
	StateSystemStopped                       = "SYSTEM_STOPPED"
	KeySchemaRegistryStencilCacheAutoRefresh = "SCHEMA_REGISTRY_STENCIL_CACHE_AUTO_REFRESH"
	KeyStencilSchemaRegistryURLs             = "STENCIL_SCHEMA_REGISTRY_URLS"
)

func (dd *daggerDriver) Plan(_ context.Context, exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	switch act.Name {
	case module.CreateAction:
		return dd.planCreate(exr, act)

	case ResetAction:
		return dd.planReset(exr, act)

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

	chartVals := mergeChartValues(&dd.conf.ChartValues, conf.ChartValues)
	conf.ChartValues = chartVals

	if conf.State == "" {
		conf.State = StateDeployed
	}
	if conf.JobState == "" {
		conf.JobState = JobStateRunning
	}

	immediately := dd.timeNow()
	conf.JarURI = dd.conf.JarURI
	exr.Resource.Labels[labelJobState] = conf.JobState
	exr.Resource.Labels[labelState] = conf.State

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
		newConf.EnvVariables[keyStreams] = string(mustMarshalJSON(newConf.Source))
		newConf.EnvVariables[keyFlinkParallelism] = fmt.Sprint(newConf.Replicas)

		//we want to update these irrespective of the user input
		newConf.ChartValues = &dd.conf.ChartValues
		newConf.JarURI = dd.conf.JarURI

		chartVals := mergeChartValues(curConf.ChartValues, newConf.ChartValues)

		// restore configs that are not user-controlled.
		newConf.DeploymentID = curConf.DeploymentID
		newConf.ChartValues = chartVals

		if newConf.State == "" {
			newConf.State = curConf.State
		}
		if newConf.JobState == "" {
			newConf.JobState = curConf.JobState
		}

		newConf.Resources = mergeResources(curConf.Resources, newConf.Resources)

		curConf = newConf

	case StopAction:
		curConf.State = StateUserStopped
		curConf.JobState = JobStateSuspended

	case StartAction:
		curConf.State = StateDeployed
		curConf.JobState = JobStateRunning
		curConf.ChartValues = &dd.conf.ChartValues
		curConf.JarURI = dd.conf.JarURI

		err := updateStencilSchemaRegistryURLsParams(curConf, act)
		if err != nil {
			return nil, err
		}
	}

	immediately := dd.timeNow()

	exr.Resource.Spec.Configs = modules.MustJSON(curConf)

	if act.Labels != nil {
		exr.Resource.Labels = act.Labels
	}
	exr.Resource.Labels[labelJobState] = curConf.JobState
	exr.Resource.Labels[labelState] = curConf.State

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

	err = updateStencilSchemaRegistryURLsParams(curConf, act)
	if err != nil {
		return nil, err
	}

	curConf.ResetOffset = resetValue

	curConf.Source = dd.consumerReset(context.Background(), *curConf, resetValue)
	curConf.EnvVariables[keyStreams] = string(mustMarshalJSON(curConf.Source))

	curConf.ChartValues = &dd.conf.ChartValues
	curConf.JarURI = dd.conf.JarURI

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

func updateStencilSchemaRegistryURLsParams(curConf *Config, act module.ActionRequest) error {
	if curConf.EnvVariables[KeySchemaRegistryStencilCacheAutoRefresh] != "" && curConf.EnvVariables[KeySchemaRegistryStencilCacheAutoRefresh] == "false" {
		stencilSchemaRegistryURLsParams := StencilSchemaRegistryURLsParams{}
		err := json.Unmarshal([]byte(act.Params), &stencilSchemaRegistryURLsParams)
		if err != nil {
			return err
		}
		curConf.EnvVariables[KeyStencilSchemaRegistryURLs] = stencilSchemaRegistryURLsParams.StencilSchemaRegistryURLs
	}
	return nil
}
