package flink

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
)

func (fd *flinkDriver) Output(ctx context.Context, exr module.ExpandedResource) (json.RawMessage, error) {
	output, err := readOutputData(exr)
	if err != nil {
		return nil, err
	}

	conf, err := readConfig(exr.Resource, exr.Resource.Spec.Configs, fd.conf)
	if err != nil {
		if errors.Is(err, errors.ErrInvalid) {
			return nil, err
		}
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	var kubeOut kubernetes.Output
	if err := json.Unmarshal(exr.Dependencies[keyKubeDependency].Output, &kubeOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}

	output.KubeCluster = kubeOut
	output.Influx = conf.Influx
	output.KubeNamespace = conf.KubeNamespace
	output.SinkKafkaStream = conf.SinkKafkaStream
	output.PrometheusURL = conf.PrometheusURL

	return modules.MustJSON(output), nil
}
