package firehose2

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
)

type Config struct {
	Stopped      bool       `json:"stopped,omitempty"`
	StopTime     *time.Time `json:"stop_time,omitempty"`
	Replicas     int        `json:"replicas" validate:"required,gte=1"`
	DeploymentID string     `json:"deployment_id,omitempty"`

	KafkaTopic         string            `json:"kafka_topic" validate:"required"`
	KafkaConsumerID    string            `json:"kafka_consumer_id" validate:"required"`
	KafkaBrokerAddress string            `json:"kafka_broker_address" validate:"required"`
	EnvVariables       map[string]string `json:"env_variables,omitempty"`
}

func readConfig(r resource.Resource, confJSON json.RawMessage) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}

	const startSequence = "0001"
	if cfg.KafkaConsumerID == "" {
		cfg.KafkaConsumerID = fmt.Sprintf("%s-%s-firehose-%s", r.Project, r.Name, startSequence)
	}

	if err := validateStruct(cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
