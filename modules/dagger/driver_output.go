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
	"github.com/goto/entropy/pkg/kube"
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

	pods, crd, err := dd.getKubeResources(ctx, kubeOut.Configs, rc.Namespace, rc.Name, conf.DeploymentID)
	if err != nil {
		return modules.MustJSON(Output{
			Error: err.Error(),
		}), nil
	}

	output.Pods = pods
	output.Namespace = conf.Namespace
	output.JobID = conf.DeploymentID
	output.JMDeployStatus = crd.JMDeployStatus
	output.JobStatus = crd.JobStatus
	output.Reconcilation = crd.Reconciliation

	state := output.JobStatus
	if state == "FINISHED" {
		state = "CANCELED"
	} else if state == "RUNNING" {
		state = "RUNNING"
	} else {
		state = "INITIALIZING"
	}
	output.State = state

	return modules.MustJSON(output), nil
}

func (dd *daggerDriver) getKubeResources(ctx context.Context, configs kube.Config, namespace, name, deploymentID string) ([]kube.Pod, kube.FlinkDeploymentStatus, error) {
	pods, err := dd.kubeGetPod(ctx, configs, namespace, map[string]string{"app": deploymentID})
	if err != nil {
		return nil, kube.FlinkDeploymentStatus{}, err
	}

	crd, err := dd.kubeGetCRD(ctx, configs, namespace, name)
	if err != nil {
		return nil, kube.FlinkDeploymentStatus{}, err
	}

	return pods, crd, nil
}
