package dagger

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube"
)

const container = "flink-main-container"

func (dd *daggerDriver) Log(ctx context.Context, res module.ExpandedResource, filter map[string]string) (<-chan module.LogChunk, error) {
	conf, err := readConfig(res, res.Spec.Configs, dd.conf)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	if filter == nil {
		filter = map[string]string{}
	}
	filter["app"] = conf.DeploymentID
	filter["container"] = container

	var flinkOut flink.Output
	if err := json.Unmarshal(res.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid flink state").WithCausef(err.Error())
	}
	kubeCl, err := kube.NewClient(ctx, flinkOut.KubeCluster.Configs)
	if err != nil {
		return nil, errors.ErrInternal.WithMsgf("failed to create new kube client on firehose driver Log").WithCausef(err.Error())
	}

	logs, err := kubeCl.StreamLogs(ctx, conf.Namespace, filter)
	if err != nil {
		return nil, err
	}

	mappedLogs := make(chan module.LogChunk)
	go func() {
		defer close(mappedLogs)
		for {
			select {
			case log, ok := <-logs:
				if !ok {
					return
				}
				mappedLogs <- module.LogChunk{Data: log.Data, Labels: log.Labels}
			case <-ctx.Done():
				return
			}
		}
	}()

	return mappedLogs, err
}
