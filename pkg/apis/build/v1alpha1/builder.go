package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func (b *Builder) Ready() bool {
	return b.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() &&
		(b.Generation == b.Status.ObservedGeneration)
}

func (b *Builder) ServiceAccount() string {
	return ""
}

func (b *Builder) Namespace() string {
	return b.ObjectMeta.Namespace
}

func (b *Builder) Tag() string {
	return b.Spec.Image
}

func (b *Builder) HasSecret() bool {
	return len(b.Spec.ImagePullSecrets) > 0
}

func (b *Builder) SecretName() string {
	if b.HasSecret() {
		return b.Spec.ImagePullSecrets[0].Name
	}
	return ""
}

func (b *Builder) getBuilderSecretVolume() v1.Volume {
	if b.HasSecret() {
		return v1.Volume{
			Name: builderPullSecretsDirName,
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: b.SecretName(),
				},
			},
		}
	} else {
		return v1.Volume{
			Name: builderPullSecretsDirName,
			VolumeSource: v1.VolumeSource{
				EmptyDir: &v1.EmptyDirVolumeSource{},
			},
		}
	}
}
