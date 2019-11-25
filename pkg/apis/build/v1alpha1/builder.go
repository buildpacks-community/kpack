package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

func (b *Builder) Ready() bool {
	return b.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() &&
		(b.Generation == b.Status.ObservedGeneration)
}

func (b *Builder) BuildBuilderSpec() BuildBuilderSpec {
	return BuildBuilderSpec{
		Image:            b.Status.LatestImage,
		ImagePullSecrets: b.Spec.ImagePullSecrets,
	}
}

func (b *Builder) ImagePullSecrets() []v1.LocalObjectReference {
	return b.Spec.ImagePullSecrets
}

func (b *Builder) Image() string {
	return b.Spec.Image
}

func (b *Builder) BuildpackMetadata() BuildpackMetadataList {
	return b.Status.BuilderMetadata
}

func (b *Builder) RunImage() string {
	return b.Status.Stack.RunImage
}
