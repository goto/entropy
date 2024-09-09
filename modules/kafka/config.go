package kafka

import (
	_ "embed"
	"encoding/json"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

var (
	//go:embed schema/config.json
	configSchemaRaw []byte
	validateConfig  = validator.FromJSONSchema(configSchemaRaw)
)

type Config struct {
	Entity        string        `json:"entity,omitempty"`
	Environment   string        `json:"environment,omitempty"`
	Landscape     string        `json:"landscape,omitempty"`
	Organization  string        `json:"organization,omitempty"`
	AdvertiseMode AdvertiseMode `json:"advertise_mode"`
	Brokers       []Broker      `json:"brokers,omitempty"`
	Type          string        `json:"type"`
}

type AdvertiseMode struct {
	Host    string `json:"host"`
	Address string `json:"address"`
}

type Broker struct {
	Name    string `json:"name"`
	Host    string `json:"host"`
	Address string `json:"address"`
}

func readConfig(res resource.Resource, confJSON json.RawMessage, dc driverConf) (*Config, error) {
	var resCfg, cfg Config

	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal").WithCausef(err.Error())
	}

	if res.Spec.Configs != nil {
		if err := json.Unmarshal(res.Spec.Configs, &resCfg); err != nil {
			return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal").WithCausef(err.Error())
		}
	}

	if cfg.Type == "" {
		if resCfg.Type != "" {
			cfg.Type = resCfg.Type
		} else {
			cfg.Type = dc.Type
		}
	}

	if cfg.Brokers == nil {
		cfg.Brokers = resCfg.Brokers
	}

	newConfJSON, err := json.Marshal(cfg)
	if err != nil {
		return nil, errors.ErrInvalid.WithMsgf("failed to marshal").WithCausef(err.Error())
	}

	if err := validateConfig(newConfJSON); err != nil {
		return nil, err
	}

	return &cfg, nil
}
