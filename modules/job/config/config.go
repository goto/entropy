package config

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/utils"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

type DriverConf struct {
	Namespace         string                       // maybe we shouldn't restrict namespace?
	RequestsAndLimits map[string]RequestsAndLimits // to use when not provided
	EnvVariables      map[string]string
}

type RequestsAndLimits struct {
	Limits   UsageSpec `json:"limits,omitempty"`
	Requests UsageSpec `json:"requests,omitempty"`
}
type UsageSpec struct {
	CPU    string `json:"cpu,omitempty" validate:"required"`
	Memory string `json:"memory,omitempty" validate:"required"`
}

type Config struct {
	Stopped    bool              `json:"stopped,omitempty"`
	Replicas   int32             `json:"replicas"`
	Namespace  string            `json:"namespace,omitempty"`
	Name       string            `json:"name,omitempty"`
	Containers []Container       `json:"containers,omitempty"`
	JobLabels  map[string]string `json:"job_labels,omitempty"`
	Volumes    []Volume          `json:"volumes,omitempty"`
}

type Volume struct {
	Name string
	Kind string
}

type Container struct {
	Name              string            `json:"name"`
	Image             string            `json:"image"`
	ImagePullPolicy   string            `json:"image_pull_policy"`
	Command           []string          `json:"command"`
	SecretsVolumes    []Secret          `json:"secrets_volumes,omitempty"`
	ConfigMapsVolumes []ConfigMap       `json:"config_maps_volumes,omitempty"`
	Limits            *UsageSpec        `json:"limits,omitempty"`
	Requests          *UsageSpec        `json:"requests,omitempty"`
	EnvConfigMaps     []string          `json:"env_config_maps,omitempty"`
	EnvVariables      map[string]string `json:"env_variables,omitempty"`
}

type Secret struct {
	Name  string `json:"name"`
	Mount string `json:"mount"`
}
type ConfigMap struct {
	Name  string `json:"name"`
	Mount string `json:"mount"`
}

var (
	//go:embed schema/config.json
	configSchemaRaw []byte
	validateConfig  = validator.FromJSONSchema(configSchemaRaw)
)
var maxJobNameLength = 47

func ReadConfig(r resource.Resource, confJSON json.RawMessage, dc DriverConf) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}
	// for each container
	rl := dc.RequestsAndLimits["default"]
	for _, c := range cfg.Containers {
		c.EnvVariables = utils.CloneAndMergeMaps(dc.EnvVariables, c.EnvVariables)
		if c.Requests == nil {
			c.Requests = &rl.Requests
		}
		if c.Limits == nil {
			c.Limits = &rl.Limits
		}
	}
	// TODO: add a container for telegraf here
	// cfg.Containers = append(cfg.Containers, createTelegrafContainer())

	if err := validateConfig(confJSON); err != nil {
		return nil, err
	}

	if len(cfg.Name) == 0 {
		cfg.Name = utils.SafeName(fmt.Sprintf("%s-%s", r.Project, r.Name), "-job", maxJobNameLength)
	} else if len(cfg.Name) > maxJobNameLength {
		return nil, errors.ErrInvalid.WithMsgf("Job name must not have more than %d chars", maxJobNameLength)
	}
	cfg.Namespace = dc.Namespace
	return &cfg, nil
}
