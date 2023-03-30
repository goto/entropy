package firehose

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"text/template"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/helm"
	"github.com/goto/entropy/pkg/kube"
)

const (
	stepReleaseCreate = "release_create"
	stepReleaseUpdate = "release_update"
	stepReleaseStop   = "release_stop"
	stepKafkaReset    = "consumer_reset"
)

const (
	chartRepo = "https://goto.github.io/charts/"
	chartName = "firehose"
	imageRepo = "gotocompany/firehose"
)

var defaultDriverConf = driverConf{
	Namespace: "firehose",
	Telegraf: Telegraf{
		Enabled: false,
	},
	ChartValues: ChartValues{
		ImageTag:        "latest",
		ChartVersion:    "0.1.3",
		ImagePullPolicy: "IfNotPresent",
	},
}

type firehoseDriver struct {
	timeNow       func() time.Time
	conf          driverConf
	kubeDeploy    kubeDeployFn
	kubeGetPod    kubeGetPodFn
	consumerReset consumerResetFn
}

type (
	kubeDeployFn    func(ctx context.Context, isCreate bool, conf kube.Config, hc helm.ReleaseConfig) error
	kubeGetPodFn    func(ctx context.Context, conf kube.Config, ns string, labels map[string]string) ([]kube.Pod, error)
	consumerResetFn func(ctx context.Context, conf Config, out kubernetes.Output, resetTo string) error
)

type driverConf struct {
	Telegraf    Telegraf    `json:"telegraf"`
	Namespace   string      `json:"namespace" validate:"required"`
	ChartValues ChartValues `json:"chart_values" validate:"required"`
}

type Output struct {
	Pods        []kube.Pod `json:"pods,omitempty"`
	Namespace   string     `json:"namespace,omitempty"`
	ReleaseName string     `json:"release_name,omitempty"`
}

type transientData struct {
	PendingSteps  []string `json:"pending_steps"`
	ResetOffsetTo string   `json:"reset_offset_to,omitempty"`
}

func (*firehoseDriver) getHelmRelease(labels map[string]string, conf Config) (*helm.ReleaseConfig, error) {
	var telegrafConfig bytes.Buffer
	telegrafConfigTpl, err := template.New("").Parse(conf.Telegraf.ConfigTpl)
	if err != nil {
		return nil, err
	} else if err := telegrafConfigTpl.Execute(&telegrafConfig, map[string]any{
		"Labels": labels,
		"Config": conf,
		"Values": conf.Telegraf.TplValues,
	}); err != nil {
		return nil, err
	}

	rc := helm.DefaultReleaseConfig()
	rc.Name = conf.DeploymentID
	rc.Repository = chartRepo
	rc.Chart = chartName
	rc.Namespace = conf.Namespace
	rc.ForceUpdate = true
	rc.Version = conf.ChartValues.ChartVersion
	rc.Values = map[string]interface{}{
		"replicaCount": conf.Replicas,
		"firehose": map[string]interface{}{
			"image": map[string]interface{}{
				"repository": imageRepo,
				"pullPolicy": conf.ChartValues.ImagePullPolicy,
				"tag":        conf.ChartValues.ImageTag,
			},
			"config": conf.EnvVariables,
		},
		"telegraf": map[string]any{
			"enabled": conf.Telegraf.Enabled,
			"image":   conf.Telegraf.Image,
			"config":  telegrafConfig.String(),
		},
	}
	return rc, nil
}

func mergeChartValues(cur, newVal *ChartValues) (*ChartValues, error) {
	if newVal == nil {
		return cur, nil
	}

	merged := ChartValues{
		ImageTag:        cur.ImageTag,
		ChartVersion:    cur.ChartVersion,
		ImagePullPolicy: cur.ImagePullPolicy,
	}

	newTag := strings.TrimSpace(newVal.ImageTag)
	if newTag != "" {
		if strings.Contains(newTag, ":") && !strings.HasPrefix(newTag, imageRepo) {
			return nil, errors.ErrInvalid.
				WithMsgf("unknown image repo: '%s', must start with '%s'", newTag, imageRepo)
		}
		merged.ImageTag = strings.TrimPrefix(newTag, imageRepo+":")
	}

	return &merged, nil
}

func readOutputData(exr module.ExpandedResource) (*Output, error) {
	var curOut Output
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

func mustJSON(v any) json.RawMessage {
	bytes, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes
}
