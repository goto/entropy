package driver

import (
	"context"

	job2 "github.com/goto/entropy/modules/job/config"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/modules/kubernetes"
	"github.com/goto/entropy/modules/utils"
	"github.com/goto/entropy/pkg/errors"
	"github.com/goto/entropy/pkg/kube/container"
	"github.com/goto/entropy/pkg/kube/job"
	"github.com/goto/entropy/pkg/kube/pod"
	"github.com/goto/entropy/pkg/kube/volume"
)

func (driver *Driver) create(ctx context.Context, r resource.Resource, config *job2.Config, out kubernetes.Output) error {
	j, err := driver.getJob(r, config)
	if err != nil {
		return err
	}

	if err := driver.CreateJob(ctx, out.Configs, j); err != nil {
		return errors.ErrInternal.WithCausef(err.Error())
	}
	return nil
}

const (
	labelsConfKey          = "labels"
	labelOrchestrator      = "orchestrator"
	labelURN               = "urn"
	labelName              = "name"
	orchestratorLabelValue = "entropy"
)

func (driver *Driver) getJob(res resource.Resource, conf *job2.Config) (*job.Job, error) {
	constantLabels := map[string]string{
		labelOrchestrator: orchestratorLabelValue,
		labelURN:          res.URN,
		labelName:         res.Name,
	}

	var volumes []volume.Volume
	var containers []container.Container
	for _, v := range conf.Volumes {
		k := volume.Secret
		if v.Kind == "config-map" {
			k = volume.ConfigMap
		}
		volumes = append(volumes, volume.Volume{
			Kind:       k,
			Name:       v.Name,
			SourceName: v.Name,
		})
	}
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
		})
	}
	p := &pod.Pod{
		Name:       conf.Name,
		Containers: containers,
		Volumes:    volumes,
	}
	j := &job.Job{
		Pod:         p,
		Name:        conf.Name,
		Namespace:   conf.Namespace,
		Labels:      utils.CloneAndMergeMaps(constantLabels, conf.JobLabels),
		Parallelism: &conf.Replicas,
	}
	return j, nil
}
