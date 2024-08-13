package e2e_test

import (
	"context"
	"time"

	"github.com/goto/entropy/pkg/kube"
	"github.com/goto/entropy/test/testbench"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kind/pkg/cluster"
)

func getRunningFirehosePods(ctx context.Context, kubeProvider *cluster.Provider, clusterName, namespace string, labels map[string]string, waitTime time.Duration) ([]kube.Pod, error) {
	host, clientCertificate, clientKey, err := testbench.GetClusterCredentials(kubeProvider, clusterName)
	if err != nil {
		return nil, err
	}

	kubeClient, err := kube.NewClient(ctx, kube.Config{
		Host:              host,
		Insecure:          true,
		ClientCertificate: clientCertificate,
		ClientKey:         clientKey,
	})
	if err != nil {
		return nil, err
	}

	time.Sleep(waitTime)
	pods, err := kubeClient.GetPodDetails(ctx, namespace, labels, func(pod v1.Pod) bool {
		return pod.Status.Phase == v1.PodRunning
	})
	if err != nil {
		return nil, err
	}

	return pods, nil
}
