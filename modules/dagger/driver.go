package dagger

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"strings"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/helm"
	"github.com/goto/entropy/pkg/kube"
)

const (
	stepReleaseCreate = "release_create"
	stepReleaseUpdate = "release_update"
	stepReleaseStop   = "release_stop"
	stepKafkaReset    = "kafka_reset"
)

const (
	chartRepo = "https://goto.github.io/charts/"
	chartName = "dagger-deployment-chart"
	imageRepo = "gotocompany/dagger"
)

const (
	labelsConfKey = "extra_labels"

	labelDeployment   = "deployment"
	labelOrchestrator = "orchestrator"
	labelURN          = "urn"
	labelName         = "name"
	labelNamespace    = "namespace"
	labelJobState     = "job_state"
	labelState        = "state"

	orchestratorLabelValue = "entropy"
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
	timeNow       func() time.Time
	conf          driverConf
	kubeDeploy    kubeDeployFn
	kubeGetPod    kubeGetPodFn
	kubeGetCRD    kubeGetCRDFn
	consumerReset consumerResetFn
}

type (
	kubeDeployFn    func(ctx context.Context, isCreate bool, conf kube.Config, hc helm.ReleaseConfig) error
	kubeGetPodFn    func(ctx context.Context, conf kube.Config, ns string, labels map[string]string) ([]kube.Pod, error)
	kubeGetCRDFn    func(ctx context.Context, conf kube.Config, ns string, name string) (kube.FlinkDeploymentStatus, error)
	consumerResetFn func(ctx context.Context, conf Config, resetTo string) []Source
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
	JMDeployStatus string     `json:"jm_deploy_status,omitempty"`
	JobStatus      string     `json:"job_status,omitempty"`
	Reconcilation  string     `json:"reconcilation,omitempty"`
	Pods           []kube.Pod `json:"pods,omitempty"`
	Namespace      string     `json:"namespace,omitempty"`
	JobID          string     `json:"job_id,omitempty"`
	Error          string     `json:"error,omitempty"`
}

type transientData struct {
	PendingSteps  []string `json:"pending_steps"`
	ResetOffsetTo string   `json:"reset_offset_to,omitempty"`
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

func readTransientData(exr module.ExpandedResource) (*transientData, error) {
	if len(exr.Resource.State.ModuleData) == 0 {
		return &transientData{}, nil
	}

	var modData transientData
	if err := json.Unmarshal(exr.Resource.State.ModuleData, &modData); err != nil {
		return nil, errors.ErrInternal.WithMsgf("corrupted transient data").WithCausef(err.Error())
	}
	return &modData, nil
}

func (dd *daggerDriver) getHelmRelease(res resource.Resource, conf Config,
	kubeOut kubernetes.Output,
) (*helm.ReleaseConfig, error) {

	entropyLabels := map[string]string{
		labelDeployment:   conf.DeploymentID,
		labelOrchestrator: orchestratorLabelValue,
	}

	otherLabels := map[string]string{
		labelURN:       res.URN,
		labelName:      res.Name,
		labelNamespace: conf.Namespace,
	}

	deploymentLabels, err := renderTpl(dd.conf.Labels, modules.CloneAndMergeMaps(res.Labels, modules.CloneAndMergeMaps(entropyLabels, otherLabels)))
	if err != nil {
		return nil, err
	}

	rc := helm.DefaultReleaseConfig()
	rc.Timeout = dd.conf.KubeDeployTimeout
	rc.Name = conf.DeploymentID
	rc.Repository = chartRepo
	rc.Chart = chartName
	rc.Namespace = conf.Namespace
	rc.ForceUpdate = true
	rc.Version = conf.ChartValues.ChartVersion

	imageRepository := dd.conf.ChartValues.ImageRepository
	if conf.ChartValues.ImageRepository != "" {
		imageRepository = conf.ChartValues.ImageRepository
	}

	conf.EnvVariables["SINK_TYPE"] = strings.ToLower(conf.EnvVariables["SINK_TYPE"])

	var programArgs []string
	for key, value := range conf.EnvVariables {
		programArgs = append(programArgs, fmt.Sprintf("%q", "--"+key), fmt.Sprintf("%q", value))
	}

	formatted := fmt.Sprintf("[%s]", strings.Join(programArgs, ","))
	encodedProgramArgs := base64.StdEncoding.EncodeToString([]byte(formatted))

	rc.Values = map[string]any{
		labelsConfKey:   modules.CloneAndMergeMaps(deploymentLabels, entropyLabels),
		"image":         imageRepository,
		"deployment_id": conf.DeploymentID,
		"configuration": map[string]any{
			"FLINK_PARALLELISM": conf.Replicas,
		},
		"projectID":      res.Project,
		"name":           modules.BuildResourceName("dagger", res.Name, res.Project, math.MaxInt),
		"team":           conf.Team,
		"flink_name":     conf.FlinkName,
		"prometheus_url": conf.PrometheusURL,
		"resources": map[string]any{
			"jobmanager": map[string]any{
				"cpu":    conf.Resources.JobManager.CPU,
				"memory": conf.Resources.JobManager.Memory,
			},
			"taskmanager": map[string]any{
				"cpu":    conf.Resources.TaskManager.CPU,
				"memory": conf.Resources.TaskManager.Memory,
			},
		},
		"jarURI":      conf.JarURI,
		"programArgs": append([]string{"--encodedArgs"}, encodedProgramArgs),
		"state":       conf.JobState,
		"namespace":   conf.Namespace,
	}

	return rc, nil
}

// TODO: move this to pkg
func renderTpl(labelsTpl map[string]string, labelsValues map[string]string) (map[string]string, error) {
	const useZeroValueForMissingKey = "missingkey=zero"

	finalLabels := map[string]string{}
	for k, v := range labelsTpl {
		var buf bytes.Buffer
		t, err := template.New("").Option(useZeroValueForMissingKey).Parse(v)
		if err != nil {
			return nil, errors.ErrInvalid.
				WithMsgf("label template for '%s' is invalid", k).WithCausef(err.Error())
		} else if err := t.Execute(&buf, labelsValues); err != nil {
			return nil, errors.ErrInvalid.
				WithMsgf("failed to render label template").WithCausef(err.Error())
		}

		// allow empty values
		//		labelVal := strings.TrimSpace(buf.String())
		//		if labelVal == "" {
		//			continue
		//		}

		finalLabels[k] = buf.String()
	}
	return finalLabels, nil
}
