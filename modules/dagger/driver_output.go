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

func (dd *daggerDriver) Output(ctx context.Context, exr module.ExpandedResource) (json.RawMessage, error) {
	output, err := readOutputData(exr)
	if err != nil {
		return nil, err
	}

	conf, err := readConfig(exr, exr.Spec.Configs, dd.conf)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	var flinkOut flink.Output
	if err := json.Unmarshal(exr.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid kube state").WithCausef(err.Error())
	}

	return dd.refreshOutput(ctx, exr.Resource, *conf, *output, flinkOut.KubeCluster)
}

func (dd *daggerDriver) refreshOutput(ctx context.Context, r resource.Resource,
	conf Config, output Output, kubeOut kubernetes.Output,
) (json.RawMessage, error) {
	rc, err := dd.getHelmRelease(r, conf, kubeOut)
	if err != nil {
		return nil, err
	}

	pods, err := dd.kubeGetPod(ctx, kubeOut.Configs, rc.Namespace, map[string]string{"app": conf.DeploymentID})
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}
	output.Pods = pods
	output.Namespace = conf.Namespace
	output.JobID = conf.DeploymentID

	crd, err := dd.kubeGetCRD(ctx, kubeOut.Configs, rc.Namespace, rc.Name)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef(err.Error())
	}

	output.JMDeployStatus = crd.JMDeployStatus
	output.JobStatus = crd.JobStatus
	output.Reconcilation = crd.Reconciliation

	return modules.MustJSON(output), nil
}
