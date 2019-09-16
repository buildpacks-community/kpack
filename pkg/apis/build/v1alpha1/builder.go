package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

func (b *Builder) Ready() bool {
	return b.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() &&
		(b.Generation == b.Status.ObservedGeneration)
}

func (b *Builder) ImageRef() BuilderImage {
	return BuilderImage{
		Image:            b.Status.LatestImage,
		ImagePullSecrets: b.Spec.ImagePullSecrets,
	}
}

func (b *Builder) SecretName() string {
	if b.HasSecret() {
		return b.Spec.ImagePullSecrets[0].Name
	}
	return ""
}

func (b *Builder) ServiceAccount() string {
	return ""
}

func (b *Builder) Namespace() string {
	return b.ObjectMeta.Namespace
}

func (b *Builder) Image() string {
	return b.Spec.Image
}

func (b *Builder) HasSecret() bool {
	return len(b.Spec.ImagePullSecrets) > 0
}

func (b *Builder) BuildpackMetadata() BuildpackMetadataList {
	return b.Status.BuilderMetadata
}
