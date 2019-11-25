package v1alpha1

import (
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (b *CustomBuilder) BuildBuilderSpec() v1alpha1.BuildBuilderSpec {
	return v1alpha1.BuildBuilderSpec{
		Image: b.Status.LatestImage,
	}
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
