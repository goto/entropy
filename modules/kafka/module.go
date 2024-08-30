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
	DriverFactory: func(_ json.RawMessage) (module.Driver, error) {
		return &kafkaDriver{}, nil
	},
}
