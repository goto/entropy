package dagger

import (
	"context"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
)

func (dd *daggerDriver) Sync(ctx context.Context, exr module.ExpandedResource) (*resource.State, error) {
	modData, err := readTransientData(exr)
	if err != nil {
		return nil, err
	}

	out, err := readOutputData(exr)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	conf, err := readConfig(exr, exr.Spec.Configs, dd.conf)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	var flinkOut flink.Output
	if err := json.Unmarshal(exr.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid flink state").WithCausef(err.Error())
	}

	finalState := resource.State{
		Status: resource.StatusPending,
		Output: exr.Resource.State.Output,
	}

	if len(modData.PendingSteps) > 0 {
		pendingStep := modData.PendingSteps[0]
		modData.PendingSteps = modData.PendingSteps[1:]

		switch pendingStep {
		case stepReleaseCreate, stepReleaseUpdate, stepReleaseStop, stepKafkaReset:
			isCreate := pendingStep == stepReleaseCreate
			if err := dd.releaseSync(ctx, exr.Resource, isCreate, *conf, flinkOut.KubeCluster); err != nil {
				return nil, err
			}
		default:
			return nil, errors.ErrInternal.WithMsgf("unknown step: '%s'", pendingStep)
		}

		// we have more pending states, so enqueue resource for another sync
		// as soon as possible.
		immediately := dd.timeNow()
		finalState.NextSyncAt = &immediately
		finalState.ModuleData = modules.MustJSON(modData)

		return &finalState, nil
	}

	finalState.NextSyncAt = conf.StopTime
	if conf.StopTime != nil && conf.StopTime.Before(dd.timeNow()) {
		conf.JobState = JobStateSuspended
		conf.State = StateSystemStopped
		if err := dd.releaseSync(ctx, exr.Resource, false, *conf, flinkOut.KubeCluster); err != nil {
			return nil, err
		}
		finalState.NextSyncAt = nil
	}

	finalOut, err := dd.refreshOutput(ctx, exr.Resource, *conf, *out, flinkOut.KubeCluster)
	if err != nil {
		return nil, err
	}
	finalState.Output = finalOut

	finalState.Status = resource.StatusCompleted
	finalState.ModuleData = nil
	return &finalState, nil

}

func (dd *daggerDriver) releaseSync(ctx context.Context, r resource.Resource,
	isCreate bool, conf Config, kubeOut kubernetes.Output,
) error {
	rc, err := dd.getHelmRelease(r, conf, kubeOut)
	if err != nil {
		return err
	}

	if err := dd.kubeDeploy(ctx, isCreate, kubeOut.Configs, *rc); err != nil {
		return errors.ErrInternal.WithCausef(err.Error())
	}
	return nil
}
