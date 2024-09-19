package dagger

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
)

const SourceKafkaConsumerAutoOffsetReset = "SOURCE_KAFKA_CONSUMER_CONFIG_AUTO_OFFSET_RESET"

func (dd *daggerDriver) Plan(_ context.Context, exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	switch act.Name {
	case module.CreateAction:
		return dd.planCreate(exr, act)
	default:
		return nil, nil
	}
}

func (dd *daggerDriver) planCreate(exr module.ExpandedResource, act module.ActionRequest) (*resource.Resource, error) {
	conf, err := readConfig(exr, act.Params, dd.conf)
	if err != nil {
		return nil, err
	}

	chartVals, err := mergeChartValues(&dd.conf.ChartValues, conf.ChartValues)
	if err != nil {
		return nil, err
	}
	conf.ChartValues = chartVals

	immediately := dd.timeNow()
	conf.JarURI = dd.conf.JarURI

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

func (dd *daggerDriver) validateHelmReleaseConfigs(expandedResource module.ExpandedResource, config Config) error {
	var flinkOut flink.Output
	if err := json.Unmarshal(expandedResource.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}

	_, err := dd.getHelmRelease(expandedResource.Resource, config, flinkOut.KubeCluster)
	return err
}
