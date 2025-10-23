package dagger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube"
)

const (
	flinkRestScheme            = "http"
	flinkRestServiceNameSuffix = "-rest"
	flinkRestServicePort       = "8081"
	flinkRestListJobsPath      = "jobs/overview"
	flinkRestExceptionPath     = "jobs/%s/exceptions"
)

type JobsOverviewResponse struct {
	Jobs []Job `json:"jobs"`
}

type Job struct {
	JobID string `json:"jid"`
}

type JobsExceptionResponse struct {
	RootException string `json:"root-exception"`
	Timestamp     int64  `json:"timestamp"`
}

func (dd *daggerDriver) Output(ctx context.Context, exr module.ExpandedResource) (json.RawMessage, error) {
	output, err := readOutputData(exr)
	if err != nil {
		return nil, err
	}

	conf, err := readConfig(exr, exr.Spec.Configs, dd.conf)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef("%s", err.Error())
	}

	var flinkOut flink.Output
	if err := json.Unmarshal(exr.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid kube state").WithCausef("%s", err.Error())
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
	output.Error = ""

	output.Exceptions = Exception{}
	if conf.State == StateDeployed && conf.JobState != JobStateSuspended {
		exc, err := dd.getFlinkExceptions(ctx, kubeOut.Configs, rc.Namespace, rc.Name)
		if err != nil {
			output.Exceptions = Exception{
				RootException: "Failed to fetch exceptions: " + err.Error(),
			}
		}
		if exc.RootException != "" {
			output.Exceptions = exc
		}
	}

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

func (dd *daggerDriver) getFlinkExceptions(ctx context.Context, configs kube.Config, namespace, name string) (Exception, error) {
	jobsResponseRaw, err := dd.kubeProxyService(ctx, configs, namespace, flinkRestScheme, name+flinkRestServiceNameSuffix, flinkRestServicePort, flinkRestListJobsPath)
	if err != nil {
		return Exception{}, err
	}

	var jobsOverview JobsOverviewResponse
	if err := json.Unmarshal(jobsResponseRaw, &jobsOverview); err != nil {
		return Exception{}, errors.ErrInternal.WithMsgf("failed to unmarshal jobs overview response").WithCausef("%s", err.Error())
	}

	var jobException Exception
	if len(jobsOverview.Jobs) > 0 {
		jobId := jobsOverview.Jobs[0].JobID
		exceptionPath := fmt.Sprintf(flinkRestExceptionPath, jobId)
		exceptionResponseRaw, err := dd.kubeProxyService(ctx, configs, namespace, flinkRestScheme, name+flinkRestServiceNameSuffix, flinkRestServicePort, exceptionPath)
		if err != nil {
			return Exception{}, err
		}

		var JobsExceptionResponse JobsExceptionResponse
		if err := json.Unmarshal(exceptionResponseRaw, &JobsExceptionResponse); err != nil {
			return Exception{}, errors.ErrInternal.WithMsgf("failed to unmarshal jobs exception response for job %s", jobId).WithCausef("%s", err.Error())
		}

		time := time.Unix(JobsExceptionResponse.Timestamp/1000, (JobsExceptionResponse.Timestamp%1000)*int64(time.Millisecond))
		jobException = Exception{
			RootException: JobsExceptionResponse.RootException,
			Timestamp:     &time,
		}
	}

	return jobException, nil
}
