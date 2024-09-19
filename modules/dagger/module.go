package dagger

import (
	"context"
	_ "embed"
	"encoding/json"
	"time"

	"github.com/goto/entropy/core/module"
	"github.com/goto/entropy/modules/flink"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/helm"
	"github.com/goto/entropy/pkg/kube"
	"github.com/goto/entropy/pkg/validator"
	"helm.sh/helm/v3/pkg/release"
	v1 "k8s.io/api/core/v1"
)

const (
	keyFlinkDependency = "flink"
)

type FlinkCRDStatus struct {
	JobManagerDeploymentStatus string `json:"jobManagerDeploymentStatus"`
	JobStatus                  string `json:"jobStatus"`
	ReconciliationStatus       string `json:"reconciliationStatus"`
}

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
			kubeGetPod: func(ctx context.Context, conf kube.Config, ns string, labels map[string]string) ([]kube.Pod, error) {
				kubeCl, err := kube.NewClient(ctx, conf)
				if err != nil {
					return nil, errors.ErrInternal.WithMsgf("failed to create new kube client on firehose driver kube get pod").WithCausef(err.Error())
				}
				return kubeCl.GetPodDetails(ctx, ns, labels, func(pod v1.Pod) bool {
					// allow pods that are in running state and are not marked for deletion
					return pod.Status.Phase == v1.PodRunning && pod.DeletionTimestamp == nil
				})
			},
			kubeGetCRD: func(ctx context.Context, conf kube.Config, ns string, name string) (kube.FlinkDeploymentStatus, error) {
				kubeCl, err := kube.NewClient(ctx, conf)
				if err != nil {
					return kube.FlinkDeploymentStatus{}, errors.ErrInternal.WithMsgf("failed to create new kube client on firehose driver kube get pod").WithCausef(err.Error())
				}
				crd, err := kubeCl.GetCRDDetails(ctx, ns, name)
				if err != nil {
					return kube.FlinkDeploymentStatus{}, err
				}
				flinkDeployment := crd.Object

				var flinkCRDStatus FlinkCRDStatus
				statusInterface, ok := flinkDeployment["status"].(map[string]interface{})
				if !ok {
					return kube.FlinkDeploymentStatus{}, errors.ErrInternal.WithMsgf("failed to convert flink deployment status to map[string]interface{}")
				}

				if jmStatus, ok := statusInterface["jobManagerDeploymentStatus"].(string); ok {
					flinkCRDStatus.JobManagerDeploymentStatus = jmStatus
				}
				if jobStatus, ok := statusInterface["jobStatus"].(map[string]interface{}); ok {
					if st, ok := jobStatus["state"].(string); ok {
						flinkCRDStatus.JobStatus = st
					}
				}
				if reconciliationStatus, ok := statusInterface["reconciliationStatus"].(map[string]interface{}); ok {
					if st, ok := reconciliationStatus["state"].(string); ok {
						flinkCRDStatus.ReconciliationStatus = st
					}
				}

				status := kube.FlinkDeploymentStatus{
					JMDeployStatus: flinkCRDStatus.JobManagerDeploymentStatus,
					JobStatus:      flinkCRDStatus.JobStatus,
					Reconciliation: flinkCRDStatus.ReconciliationStatus,
				}
				return status, nil

			}}, nil
	},
}
