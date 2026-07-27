package flink

import (
	_ "embed"
	"encoding/json"

	"go.uber.org/zap"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/pkg/errors"
)

const (
	keyKubeDependency = "kube_cluster"
)

var Module = module.Descriptor{
	Kind: "flink",
	Actions: []module.ActionDesc{
		{
			Name: module.CreateAction,
		},
		{
			Name: module.UpdateAction,
		},
	},
	DriverFactory: func(conf json.RawMessage) (module.Driver, error) {
		var fd flinkDriver
		if len(conf) > 0 {
			if err := json.Unmarshal(conf, &fd); err != nil {
				return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal module config: %v", err)
			}
		} else {
			zap.L().Warn("flink module has empty config; driver initialised with zero values")
		}
		return &fd, nil
	},
}
