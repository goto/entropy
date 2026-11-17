package dagger

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube"
)

func (dd *daggerDriver) WebUIProxy(ctx context.Context, res module.ExpandedResource) (http.Handler, error) {
	conf, err := readConfig(res, res.Spec.Configs, dd.conf)
	if err != nil {
		return nil, errors.ErrInternal.WithCausef("%s", err.Error())
	}

	var flinkOut flink.Output
	if err := json.Unmarshal(res.Dependencies[keyFlinkDependency].Output, &flinkOut); err != nil {
		return nil, errors.ErrInternal.WithMsgf("invalid flink state").WithCausef("%s", err.Error())
	}

	kubeCl, err := kube.NewClient(ctx, flinkOut.KubeCluster.Configs)
	if err != nil {
		return nil, errors.ErrInternal.WithMsgf("failed to create new kube client on dagger driver WebUIProxy").WithCausef("%s", err.Error())
	}

	return kubeCl.ProxyServiceHandler(conf.Namespace, flinkRestScheme, conf.DeploymentID+flinkRestServiceNameSuffix, flinkRestServicePort)
}
