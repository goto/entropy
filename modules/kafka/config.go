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
	cfg := Config{
		Type:         dc.Type,
		Entity:       dc.Entity,
		Organization: dc.Organization,
		Landscape:    dc.Landscape,
		Environment:  dc.Environment,
	}

	if res.Spec.Configs != nil {
		if err := json.Unmarshal(res.Spec.Configs, &cfg); err != nil {
			return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal").WithCausef(err.Error())
		}
	}

	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal").WithCausef(err.Error())
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
