package dagger

import (
	"context"
	_ "embed"
	"encoding/json"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/helm"
	"github.com/goto/entropy/pkg/kube"
	"github.com/goto/entropy/pkg/validator"
	"helm.sh/helm/v3/pkg/release"
)

const (
	keyFlinkDependency = "flink"
)

var Module = module.Descriptor{
	Kind: "dagger",
	Dependencies: map[string]string{
		keyFlinkDependency: flink.Module.Kind,
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
			conf:    conf,
			timeNow: time.Now,
			kubeDeploy: func(_ context.Context, isCreate bool, kubeConf kube.Config, hc helm.ReleaseConfig) error {
				canUpdate := func(rel *release.Release) bool {
					curLabels, ok := rel.Config[labelsConfKey].(map[string]any)
					if !ok {
						return false
					}
					newLabels, ok := hc.Values[labelsConfKey].(map[string]string)
					if !ok {
						return false
					}

					isManagedByEntropy := curLabels[labelOrchestrator] == orchestratorLabelValue
					isSameDeployment := curLabels[labelDeployment] == newLabels[labelDeployment]

					return isManagedByEntropy && isSameDeployment
				}

				helmCl := helm.NewClient(&helm.Config{Kubernetes: kubeConf})
				_, errHelm := helmCl.Upsert(&hc, canUpdate)
				return errHelm
			},
			kubeGetPod: nil,
		}, nil
	},
}
