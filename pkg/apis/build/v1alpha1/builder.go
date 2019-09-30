package v1alpha1

import (
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

func (b *Builder) ImagePullSecrets() []string {
	var secrets []string
	for _, s := range b.Spec.ImagePullSecrets {
		secrets = append(secrets, s.Name)
	}
	return secrets
}

func (b *Builder) Image() string {
	return b.Spec.Image
}

func (b *Builder) BuildpackMetadata() BuildpackMetadataList {
	return b.Status.BuilderMetadata
}
