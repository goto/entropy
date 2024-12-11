package flink

import (
	_ "embed"
	"encoding/json"

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
		fd := &flinkDriver{}
		err := json.Unmarshal(conf, &fd)
		if err != nil {
			return nil, errors.ErrInvalid.WithMsgf("failed to unmarshal module config: %v", err)
		}
		return fd, nil
	},
}
