package kubernetes

import (
	_ "embed"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/errors"
)

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
		var kd kubeDriver
		if len(conf) > 0 {
			if err := json.Unmarshal(conf, &kd); err != nil {
				return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal module config: %v", err)
			}
		} else {
			zap.L().Warn("kubernetes module has empty config; driver initialised with zero values")
		}
		return &kd, nil
	},
}
