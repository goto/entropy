package pod

import (
	"github.com/goto/entropy/pkg/kube/container"
	"github.com/goto/entropy/pkg/kube/volume"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Pod struct {
	Name       string
	Containers []container.Container
	Volumes    []volume.Volume
}

func (p Pod) Template() corev1.PodTemplateSpec {
	var containers []corev1.Container
	for _, c := range p.Containers {
		containers = append(containers, c.Template())
	}
	var volumes []corev1.Volume
	for _, v := range p.Volumes {
		var vSource corev1.VolumeSource
		switch v.Kind {
		case volume.Secret:
			vSource.Secret = &corev1.SecretVolumeSource{
				SecretName: v.SourceName,
			}
		case volume.ConfigMap:
			vSource.ConfigMap = &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: v.SourceName},
			}
		}
		volumes = append(volumes, corev1.Volume{
			Name:         v.Name,
			VolumeSource: corev1.VolumeSource{},
		})
	}
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Name: p.Name},
		Spec: corev1.PodSpec{
			Containers: containers,
			Volumes:    volumes,
		},
	}
}
