package buildpod

import (
	"fmt"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
)

type ServiceBindings []ServiceBinding

type ServiceBinding struct {
	Name      string
	SecretRef *v1.LocalObjectReference
}

func (sb ServiceBindings) AppProjections(mountDir string) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	for _, s := range sb {
		if s.SecretRef != nil {
			secretVolume := fmt.Sprintf("service-binding-secret-%s", s.Name)
			volumes = append(volumes,
				corev1.Volume{
					Name: secretVolume,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: s.SecretRef.Name,
						},
					},
				},
			)
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      secretVolume,
					MountPath: fmt.Sprintf("%s/bindings/%s", mountDir, s.Name),
					ReadOnly:  true,
				},
			)
		}
	}

	return volumes, volumeMounts
}

type V1Alpha1ServiceBindings []v1alpha1.Binding

func (sb V1Alpha1ServiceBindings) AppProjections(mountDir string) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	for _, binding := range sb {
		metadataVolume := fmt.Sprintf("binding-metadata-%s", binding.Name)
		volumes = append(volumes,
			corev1.Volume{
				Name: metadataVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: *binding.MetadataRef,
					},
				},
			},
		)
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      metadataVolume,
				MountPath: fmt.Sprintf("%s/bindings/%s/metadata", mountDir, binding.Name),
				ReadOnly:  true,
			},
		)
		if binding.SecretRef != nil {
			secretVolume := fmt.Sprintf("binding-secret-%s", binding.Name)
			volumes = append(volumes,
				corev1.Volume{
					Name: secretVolume,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: binding.SecretRef.Name,
						},
					},
				},
			)
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      secretVolume,
					MountPath: fmt.Sprintf("%s/bindings/%s/secret", mountDir, binding.Name),
					ReadOnly:  true,
				},
			)
		}
	}

	return volumes, volumeMounts
}
