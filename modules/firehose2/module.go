package firehose2

import (
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/kubernetes"
)

const keyKubeDependency = "kube_cluster"

const (
	ResetAction   = "reset"
	UpgradeAction = "upgrade"
)

var defaultDriverConf = driverConf{
	ChartRepository: "https://odpf.github.io/charts/",
	ChartName:       "firehose",
	ChartVersion:    "0.1.3",
	ImageRepository: "gotocompany/firehose",
	ImageName:       "firehose",
	ImageTag:        "latest",
	Namespace:       "firehose",
	ImagePullPolicy: "IfNotPresent",
}

var Module = module.Descriptor{
	Kind: "firehose2",
	Dependencies: map[string]string{
		keyKubeDependency: kubernetes.Module.Kind,
	},
	Actions: []module.ActionDesc{
		{
			Name:        module.CreateAction,
			Description: "Creates a new firehose",
		},
		{
			Name:        module.UpdateAction,
			Description: "Update all configurations of firehose",
		},
		{
			Name:        ResetAction,
			Description: "Stop firehose, reset consumer group, restart",
		},
		{
			Name:        UpgradeAction,
			Description: "Upgrade firehose version",
		},
	},
	DriverFactory: func(confJSON json.RawMessage) (module.Driver, error) {
		conf := defaultDriverConf // clone the default value
		if err := json.Unmarshal(confJSON, &conf); err != nil {
			return nil, err
		} else if err := conf.sanitise(); err != nil {
			return nil, err
		}

		return &firehoseDriver{
			conf: conf,
		}, nil
	},
}

type driverConf struct {
	ChartName       string `json:"chart_name,omitempty" validate:"required"`
	ChartVersion    string `json:"chart_version,omitempty" validate:"required"`
	ChartRepository string `json:"chart_repository,omitempty" validate:"required"`

	Namespace       string `json:"namespace,omitempty" validate:"required"`
	ImagePullPolicy string `json:"image_pull_policy,omitempty" validate:"required"`

	ImageTag        string `json:"image_tag,omitempty" validate:"required"`
	ImageName       string `json:"image_name,omitempty" validate:"required"`
	ImageRepository string `json:"image_repository,omitempty" validate:"required"`
}

func (dc *driverConf) sanitise() error {
	return validateStruct(dc)
}
