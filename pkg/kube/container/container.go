package container

import (
	corev1 "k8s.io/api/core/v1"
)

type Container struct {
	Image           string
	Name            string
	EnvConfigMaps   []string
	Command         []string
	EnvMap          map[string]string
	Args            []string
	ImagePullPolicy string
	VolumeMounts    []VolumeMount
}
type VolumeMount struct {
	Name      string
	MountPath string
}

func (c Container) Template() corev1.Container {
	var env []corev1.EnvVar
	for k, v := range c.EnvMap {
		env = append(env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	var envFrom []corev1.EnvFromSource
	for _, configMap := range c.EnvConfigMaps {
		envFrom = append(envFrom, corev1.EnvFromSource{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: configMap},
			},
		})
	}
	var mounts []corev1.VolumeMount
	for _, v := range c.VolumeMounts {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      v.Name,
			MountPath: v.MountPath,
		})
	}
	return corev1.Container{
		Name:            c.Name,
		Image:           c.Image,
		Command:         c.Command,
		Args:            c.Args,
		EnvFrom:         envFrom,
		Env:             env,
		Resources:       corev1.ResourceRequirements{},
		VolumeMounts:    mounts,
		ImagePullPolicy: corev1.PullPolicy(c.ImagePullPolicy),
	}
}
