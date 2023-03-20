package firehose2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/goto/entropy/core/module"
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

var defaultDriverConf = driverConf{
	Namespace: "firehose",
	Telegraf: map[string]any{
		"enabled": false,
	},
	ChartValues: chartValues{
		ImageTag:        "latest",
		ChartVersion:    "0.1.3",
		ImagePullPolicy: "IfNotPresent",
	},
}

type firehoseDriver struct {
	conf       driverConf
	kubeDeploy kubeDeployFn
	kubeGetPod kubeGetPodFn
}

type (
	kubeDeployFn func(ctx context.Context, isCreate bool, conf kube.Config, hc helm.ReleaseConfig) error
	kubeGetPodFn func(ctx context.Context, conf kube.Config, ns string, labels map[string]string) ([]kube.Pod, error)
)

type driverConf struct {
	Telegraf    map[string]any `json:"telegraf"`
	Namespace   string         `json:"namespace" validate:"required"`
	ChartValues chartValues    `json:"chart_values" validate:"required"`
}

type chartValues struct {
	ImageTag        string `json:"image_tag" validate:"required"`
	ChartVersion    string `json:"chart_version" validate:"required"`
	ImagePullPolicy string `json:"image_pull_policy" validate:"required"`
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

func (fd *firehoseDriver) getHelmRelease(conf Config) *helm.ReleaseConfig {
	const (
		chartRepo = "https://odpf.github.io/charts/"
		chartName = "firehose"
		imageRepo = "odpf/firehose"
	)

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
		"telegraf": fd.conf.Telegraf,
	}
	return rc
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

func validateStruct(structVal any) error {
	err := validator.New().Struct(structVal)
	if err != nil {
		var fields []string
		for _, fieldError := range err.(validator.ValidationErrors) {
			fields = append(fields, fmt.Sprintf("%s: %s", fieldError.Field(), fieldError.Tag()))
		}
		return errors.ErrInvalid.
			WithMsgf("invalid values for fields").
			WithCausef(strings.Join(fields, ", "))
	}
	return nil
}