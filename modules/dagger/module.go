package dagger

import (
	_ "embed"
	"encoding/json"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/validator"
)

const (
	keyKubeDependency = "kube_cluster"
)

var Module = module.Descriptor{
	Kind: "dagger",
	Dependencies: map[string]string{
		keyKubeDependency: kubernetes.Module.Kind,
	},
	Actions: []module.ActionDesc{
		{
			Name:        module.CreateAction,
			Description: "Creates a new dagger",
		},
	},
	DriverFactory: func(confJSON json.RawMessage) (module.Driver, error) {
		conf := defaultDriverConf // clone the default value
		if err := json.Unmarshal(confJSON, &conf); err != nil {
			return nil, err
		} else if err := validator.TaggedStruct(conf); err != nil {
			return nil, err
		}

		return &daggerDriver{
			conf:       conf,
			timeNow:    time.Now,
			kubeDeploy: nil,
			kubeGetPod: nil,
		}, nil
	},
}
