package flink

import (
	_ "embed"
	"encoding/json"

	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

var (
	//go:embed schema/config.json
	configSchemaRaw []byte

	validateConfig = validator.FromJSONSchema(configSchemaRaw)
)

type Influx struct {
	URL          string `json:"url,omitempty"`
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	DatabaseName string `json:"database_name,omitempty"`
}

type Config struct {
	KubeNamespace   string   `json:"kube_namespace,omitempty"`
	Influx          Influx   `json:"influx,omitempty"`
	SinkKafkaStream string   `json:"sink_kafka_stream,omitempty"`
	PrometheusURL   string   `json:"prometheus_url,omitempty"`
	FlinkName       string   `json:"flink_name,omitempty"`
	ExtraStreams    []string `json:"extra_streams,omitempty"`
}

func readConfig(confJSON json.RawMessage, dc driverConf) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}

	if cfg.Influx.URL == "" {
		cfg.Influx.URL = dc.Influx.URL
		cfg.Influx.Username = dc.Influx.Username
		cfg.Influx.Password = dc.Influx.Password
		cfg.Influx.DatabaseName = dc.Influx.DatabaseName
	}

	if cfg.SinkKafkaStream == "" {
		cfg.SinkKafkaStream = dc.SinkKafkaStream
	}

	if cfg.KubeNamespace == "" {
		cfg.KubeNamespace = dc.KubeNamespace
	}

	if cfg.PrometheusURL == "" {
		cfg.PrometheusURL = dc.PrometheusURL
	}

	if cfg.FlinkName == "" {
		cfg.FlinkName = dc.FlinkName
	}

	return &cfg, nil
}
