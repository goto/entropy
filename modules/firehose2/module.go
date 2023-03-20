package firehose2

import (
	"context"
	_ "embed"
	"encoding/json"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/pkg/helm"
	"github.com/goto/entropy/pkg/kube"
)

const (
	keyKubeDependency = "kube_cluster"

	ResetAction   = "reset"
	UpgradeAction = "upgrade"
)

var Module = module.Descriptor{
	Kind: "firehose2",
	Dependencies: map[string]string{
		keyKubeDependency: kubernetes.Module.Kind,
	},
	Actions: []module.ActionDesc{
		{
			Name:        module.CreateAction,
			Description: "Creates a new firehose",
		},
		{
			Name:        module.UpdateAction,
			Description: "Update all configurations of firehose",
		},
		{
			Name:        ResetAction,
			Description: "Stop firehose, reset consumer group, restart",
		},
		{
			Name:        UpgradeAction,
			Description: "Upgrade firehose version",
		},
	},
	DriverFactory: func(confJSON json.RawMessage) (module.Driver, error) {
		conf := defaultDriverConf // clone the default value
		if err := json.Unmarshal(confJSON, &conf); err != nil {
			return nil, err
		} else if err := validateStruct(conf); err != nil {
			return nil, err
		}

		return &firehoseDriver{
			conf: conf,
			kubeDeploy: func(_ context.Context, isCreate bool, kubeConf kube.Config, hc helm.ReleaseConfig) error {
				helmCl := helm.NewClient(&helm.Config{Kubernetes: kubeConf})

				var helmErr error
				if isCreate {
					_, helmErr = helmCl.Create(&hc)
				} else {
					_, helmErr = helmCl.Update(&hc)
				}
				return helmErr
			},
			kubeGetPod: func(ctx context.Context, conf kube.Config, ns string, labels map[string]string) ([]kube.Pod, error) {
				kubeCl := kube.NewClient(conf)
				return kubeCl.GetPodDetails(ctx, ns, labels)
			},
		}, nil
	},
}
