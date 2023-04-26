package kubernetes

import (
	_ "embed"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/errors"
)

const tolerationKey = "tolerations"

var Module = module.Descriptor{
	Kind: "kubernetes",
	Actions: []module.ActionDesc{
		{
			Name: module.CreateAction,
		},
		{
			Name: module.UpdateAction,
		},
	},
	DriverFactory: func(conf json.RawMessage) (module.Driver, error) {
		configs := map[string]map[string][]Toleration{}
		err := json.Unmarshal(conf, &configs)
		if err != nil {
			return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal module config: %v", err)
		}

		kd := &kubeDriver{}

		if configs[tolerationKey] != nil {
			kd.Tolerations = configs[tolerationKey]
		}

		return kd, nil
	},
}
