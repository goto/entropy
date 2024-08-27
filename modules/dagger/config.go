package dagger

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

const helmReleaseNameMaxLength = 53

var (
	//go:embed schema/config.json
	configSchemaRaw []byte

	validateConfig = validator.FromJSONSchema(configSchemaRaw)
)

type UsageSpec struct {
	CPU    string `json:"cpu,omitempty" validate:"required"`
	Memory string `json:"memory,omitempty" validate:"required"`
}

type Resources struct {
	TaskManager UsageSpec `json:"taskmanager,omitempty"`
	JobManager  UsageSpec `json:"jobmanager,omitempty"`
}

type Stream struct {
	SourceKafkaName string `json:"source_kafka_name,omitempty"`
	ConsumerGroupId string `json:"consumer_group_id,omitempty"`
}

type Config struct {
	Resources    Resources         `json:"resources,omitempty"`
	FlinkName    string            `json:"flink_name,omitempty"`
	DeploymentID string            `json:"deployment_id,omitempty"`
	Streams      []Stream          `json:"streams,omitempty"`
	JobId        string            `json:"job_id,omitempty"`
	Savepoint    any               `json:"savepoint,omitempty"`
	EnvVariables map[string]string `json:"env_variables,omitempty"`
	ChartValues  *ChartValues      `json:"chart_values,omitempty"`
	Deleted      bool              `json:"deleted,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Replicas     int               `json:"replicas"`
}

type ChartValues struct {
	ChartVersion string `json:"chart_version" validate:"required"`
}

func readConfig(r module.ExpandedResource, confJSON json.RawMessage, dc driverConf) (*Config, error) {
	var cfg Config
	err := json.Unmarshal(confJSON, &cfg)
	if err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}

	cfg.EnvVariables = modules.CloneAndMergeMaps(dc.EnvVariables, cfg.EnvVariables)

	if cfg.Replicas <= 0 {
		cfg.Replicas = 1
	}

	if err := validateConfig(confJSON); err != nil {
		return nil, err
	}

	// note: enforce the kubernetes deployment name length limit.
	if len(cfg.DeploymentID) == 0 {
		cfg.DeploymentID = modules.SafeName(fmt.Sprintf("%s-%s", r.Project, r.Name), "-dagger", helmReleaseNameMaxLength)
	} else if len(cfg.DeploymentID) > helmReleaseNameMaxLength {
		return nil, errors.ErrInvalid.WithMsgf("deployment_id must not have more than 53 chars")
	}

	cfg.Resources = dc.Resources

	if cfg.Namespace == "" {
		//TODO: add error handling
		kubeUrn := r.Spec.Dependencies[keyKubeDependency]
		ns := dc.Namespace[kubeUrn]
		cfg.Namespace = ns
	}

	return &cfg, nil
}
