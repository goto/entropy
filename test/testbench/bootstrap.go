package testbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	entropyv1beta1 "github.com/goto/entropy/proto/gotocompany/entropy/v1beta1"
	"sigs.k8s.io/kind/pkg/cluster"
)

func BootstrapKubernetesModule(ctx context.Context, client entropyv1beta1.ModuleServiceClient, testDataPath string) error {
	moduleData, err := os.ReadFile(testDataPath + "/module/kubernetes_module.json")
	if err != nil {
		return err
	}

	var moduleConfig *entropyv1beta1.Module
	if err = json.Unmarshal(moduleData, &moduleConfig); err != nil {
		return err
	}

	project := moduleConfig.Project
	for i := 0; i < 3; i++ {
		moduleConfig.Project = fmt.Sprintf("%s-%d", project, i)
		if _, err := client.CreateModule(ctx, &entropyv1beta1.CreateModuleRequest{
			Module: moduleConfig,
		}); err != nil {
			return err
		}
	}

	return nil
}

func BootstrapFirehoseModule(ctx context.Context, client entropyv1beta1.ModuleServiceClient, testDataPath string) error {
	moduleData, err := os.ReadFile(testDataPath + "/module/firehose_module.json")
	if err != nil {
		return err
	}

	var moduleConfig *entropyv1beta1.Module
	if err = json.Unmarshal(moduleData, &moduleConfig); err != nil {
		return err
	}

	project := moduleConfig.Project
	for i := 0; i < 3; i++ {
		moduleConfig.Project = fmt.Sprintf("%s-%d", project, i)

		if _, err := client.CreateModule(ctx, &entropyv1beta1.CreateModuleRequest{
			Module: moduleConfig,
		}); err != nil {
			return err
		}
	}

	return nil
}

func BootstrapKubernetesResource(ctx context.Context, client entropyv1beta1.ResourceServiceClient, kubeProvider *cluster.Provider, testDataPath string) error {
	resourceData, err := os.ReadFile(testDataPath + "/resource/kubernetes_resource.json")
	if err != nil {
		return err
	}

	host, clientCertificate, clientKey, err := GetClusterCredentials(kubeProvider, TestClusterName)
	if err != nil {
		return err
	}

	type Config struct {
		Host              string `json:"host"`
		Insecure          bool   `json:"insecure"`
		Timeout           uint   `json:"timeout"`
		ClientCertificate string `json:"client_certificate"`
		ClientKey         string `json:"client_key"`
	}

	type Spec struct {
		Configs     Config            `json:"configs"`
		Depedencies map[string]string `json:"dependencies"`
	}

	type SpecConfig struct {
		Specs Spec `json:"spec"`
	}

	specConfig := SpecConfig{
		Specs: Spec{
			Configs: Config{
				Host:              host,
				Insecure:          true,
				ClientCertificate: clientCertificate,
				ClientKey:         clientKey,
			},
		},
	}

	specData, err := json.Marshal(specConfig)
	if err != nil {
		return err
	}

	var resourceConfig *entropyv1beta1.Resource
	if err = json.Unmarshal(resourceData, &resourceConfig); err != nil {
		return err
	}

	if err = json.Unmarshal(specData, &resourceConfig); err != nil {
		return err
	}

	project := resourceConfig.Project
	for i := 0; i < 3; i++ {
		resourceConfig.Project = fmt.Sprintf("%s-%d", project, i)

		if _, err := client.CreateResource(ctx, &entropyv1beta1.CreateResourceRequest{
			Resource: resourceConfig,
		}); err != nil {
			return errors.New(resourceConfig.Spec.Configs.GetStringValue())
		}
	}

	return nil
}
