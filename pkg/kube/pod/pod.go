package pod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/goto/entropy/pkg/kube/container"
	"github.com/goto/entropy/pkg/kube/volume"
)

type Pod struct {
	Name       string
	Containers []container.Container
	Volumes    []volume.Volume
	Labels     map[string]string
}

func (p Pod) Template() corev1.PodTemplateSpec {
	var containers []corev1.Container
	for _, c := range p.Containers {
		containers = append(containers, c.Template())
	}
	var volumes []corev1.Volume
	for _, v := range p.Volumes {
		volumes = append(volumes, v.GetPodVolume())
	}
	volumes = append(volumes, corev1.Volume{
		Name:         "shared-data",
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	})
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:   p.Name,
			Labels: p.Labels,
		},
		Spec: corev1.PodSpec{
			Containers:    containers,
			Volumes:       volumes,
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}
