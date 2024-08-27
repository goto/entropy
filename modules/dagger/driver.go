package dagger

import (
	"context"
	"encoding/json"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/helm"
	"github.com/goto/entropy/pkg/kube"
)

const (
	stepReleaseCreate = "release_create"
)

const (
	chartRepo = "https://goto.github.io/charts/"
	chartName = "dagger"
	imageRepo = "gotocompany/dagger"
)

const (
	labelsConfKey = "labels"
)

const defaultKey = "default"

var defaultDriverConf = driverConf{
	Namespace: map[string]string{
		defaultKey: "dagger",
	},
	ChartValues: ChartValues{
		ChartVersion: "0.1.0",
	},
}

type daggerDriver struct {
	timeNow    func() time.Time
	conf       driverConf
	kubeDeploy kubeDeployFn
	kubeGetPod kubeGetPodFn
}

type (
	kubeDeployFn func(ctx context.Context, isCreate bool, conf kube.Config, hc helm.ReleaseConfig) error
	kubeGetPodFn func(ctx context.Context, conf kube.Config, ns string, labels map[string]string) ([]kube.Pod, error)
)

type driverConf struct {
	// Labels to be injected to the chart during deployment. Values can be Go templates.
	Labels map[string]string `json:"labels,omitempty"`

	// Namespace is the kubernetes namespace where firehoses will be deployed.
	Namespace map[string]string `json:"namespace" validate:"required"`

	// ChartValues is the chart and image version information.
	ChartValues ChartValues `json:"chart_values" validate:"required"`

	EnvVariables map[string]string `json:"env_variables,omitempty"`

	Resources Resources `json:"resources" validate:"required"`

	JarURI string `json:"jar_uri" validate:"required"`

	// timeout value for a kube deployment run
	KubeDeployTimeout int `json:"kube_deploy_timeout_seconds"`
}

type Output struct {
	State          string     `json:"state,omitempty"`
	JMDeployStatus string     `json:"jm_deploy_status,omitempty"`
	JobStatus      string     `json:"job_status,omitempty"`
	Namespace      string     `json:"namespace,omitempty"`
	ReleaseName    string     `json:"release_name,omitempty"`
	Pods           []kube.Pod `json:"pods,omitempty"`
}

type transientData struct {
	PendingSteps []string `json:"pending_steps"`
}

func (dd *daggerDriver) getHelmRelease(_ resource.Resource, conf Config,
	_ kubernetes.Output,
) (*helm.ReleaseConfig, error) {

	rc := helm.DefaultReleaseConfig()
	rc.Timeout = dd.conf.KubeDeployTimeout
	rc.Name = conf.DeploymentID
	rc.Repository = chartRepo
	rc.Chart = chartName
	rc.Namespace = conf.Namespace
	rc.ForceUpdate = true
	rc.Version = conf.ChartValues.ChartVersion

	rc.Values = map[string]any{
		labelsConfKey:  dd.conf.Labels,
		"replicaCount": conf.Replicas,
		"dagger": map[string]any{
			"config":    conf.EnvVariables,
			"resources": conf.Resources,
		},
	}

	return rc, nil
}

func mergeChartValues(cur, newVal *ChartValues) (*ChartValues, error) {
	if newVal == nil {
		return cur, nil
	}

	merged := ChartValues{
		ChartVersion: newVal.ChartVersion,
	}

	return &merged, nil
}

func readOutputData(exr module.ExpandedResource) (*Output, error) {
	var curOut Output
	if len(exr.Resource.State.Output) == 0 {
		return &curOut, nil
	}
	if err := json.Unmarshal(exr.Resource.State.Output, &curOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted output").WithCausef(err.Error())
	}
	return &curOut, nil
}
