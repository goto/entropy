package driver

import (
	"context"
	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/job/config"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/modules/utils"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube/container"
	"github.com/goto/entropy/pkg/kube/job"
	"github.com/goto/entropy/pkg/kube/pod"
	"github.com/goto/entropy/pkg/kube/volume"
)

const (
	labelOrchestrator            = "orchestrator"
	labelName                    = "name"
	orchestratorLabelValue       = "entropy"
	backoffLimit           int32 = 0
)

func (driver *Driver) create(ctx context.Context, r resource.Resource, config *config.Config, out kubernetes.Output) error {
	j := getJob(r, config)
	if err := driver.CreateJob(ctx, out.Configs, j); err != nil {
		return errors.ErrInternal.WithCausef(err.Error())
	}
	return nil
}

func getJob(res resource.Resource, conf *config.Config) *job.Job {
	constantLabels := map[string]string{
		labelOrchestrator: orchestratorLabelValue,
		labelName:         res.Name,
	}

	var volumes []volume.Volume
	for _, v := range conf.Volumes {
		k := volume.Secret
		if v.Kind == "configMap" {
			k = volume.ConfigMap
		}
		volumes = append(volumes, volume.Volume{
			Kind:       k,
			Name:       v.Name,
			SourceName: v.Name,
		})
	}
	var containers []container.Container
	for _, c := range conf.Containers {
		var vm []container.VolumeMount
		for _, s := range c.SecretsVolumes {
			vm = append(vm, container.VolumeMount{
				Name:      s.Name,
				MountPath: s.Mount,
			})
		}
		for _, cm := range c.ConfigMapsVolumes {
			vm = append(vm, container.VolumeMount{
				Name:      cm.Name,
				MountPath: cm.Mount,
			})
		}
		containers = append(containers, container.Container{
			Image:           c.Image,
			Name:            c.Name,
			EnvConfigMaps:   c.EnvConfigMaps,
			Command:         c.Command,
			EnvMap:          c.EnvVariables,
			ImagePullPolicy: c.ImagePullPolicy,
			VolumeMounts:    vm,
			Requests:        map[string]string{"cpu": c.Requests.CPU, "memory": c.Requests.Memory},
			Limits:          map[string]string{"cpu": c.Limits.CPU, "memory": c.Limits.Memory},
		})
	}
	p := &pod.Pod{
		Name:       conf.Name,
		Containers: containers,
		Volumes:    volumes,
	}
	limit := backoffLimit
	j := &job.Job{
		Pod:         p,
		Name:        conf.Name,
		Namespace:   conf.Namespace,
		Labels:      utils.CloneAndMergeMaps(constantLabels, conf.JobLabels),
		Parallelism: &conf.Replicas,
		BackOffList: &limit,
	}
	return j
}
