package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (b *CustomBuilder) BuildBuilderSpec() v1alpha1.BuildBuilderSpec {
	return v1alpha1.BuildBuilderSpec{
		Image:            b.Status.LatestImage,
	}
}

// FIXME : is this correct?
func (b *CustomBuilder) Image() string {
	return b.Spec.Tag
}

func (b *CustomBuilder) ImagePullSecrets() []v1.LocalObjectReference {
	return nil
}

func (b *CustomBuilder) Ready() bool {
	return b.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() && (b.Generation == b.Status.ObservedGeneration)
}

func (b *CustomBuilder) BuildpackMetadata() v1alpha1.BuildpackMetadataList {
	return b.Status.BuilderMetadata
}

func (b *CustomBuilder) RunImage() string {
	return b.Status.Stack.RunImage
}

func (b *CustomBuilder) Stack() string {
	return b.Status.Stack.ID
}
