package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
)

func (b *Builder) ImagePullSecrets() []v1.LocalObjectReference {
	return b.Spec.ImagePullSecrets
}

func (b *Builder) Image() string {
	return b.Spec.Image
}
