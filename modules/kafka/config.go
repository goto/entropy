package kafka

import (
	_ "embed"
	"encoding/json"

	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/validator"
)

var (
	//go:embed schema/config.json
	configSchemaRaw []byte
	validateConfig  = validator.FromJSONSchema(configSchemaRaw)
)

type Config struct {
	Entity        string        `json:"entity"`
	Environment   string        `json:"environment"`
	Landscape     string        `json:"landscape"`
	Organization  string        `json:"organization"`
	AdvertiseMode AdvertiseMode `json:"advertise_mode"`
	Brokers       []Broker      `json:"brokers"`
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

func readConfig(confJSON json.RawMessage) (*Config, error) {
	var cfg Config

	if err := json.Unmarshal(confJSON, &cfg); err != nil {
		return nil, errors.ErrInvalid.WithMsgf("invalid config json").WithCausef(err.Error())
	}

	if err := validateConfig(confJSON); err != nil {
		return nil, err
	}

	return &cfg, nil
}
