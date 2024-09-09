package kafka

import (
	"encoding/json"

	"github.com/goto/entropy/core/module"
)

var Module = module.Descriptor{
	Kind: "kafka",
	Actions: []module.ActionDesc{
		{
			Name: module.CreateAction,
		},
		{
			Name: module.UpdateAction,
		},
	},
	DriverFactory: func(confJSON json.RawMessage) (module.Driver, error) {
		conf := defaultDriverConf
		if err := json.Unmarshal(confJSON, &conf); err != nil {
			return nil, err
		}

		return &kafkaDriver{
			conf: conf,
		}, nil
	},
}
