package v1alpha2

import corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"

type BuilderResource interface {
	GetName() string
	BuildBuilderSpec() corev1alpha1.BuildBuilderSpec
	Ready() bool
	BuildpackMetadata() corev1alpha1.BuildpackMetadataList
	RunImage() string
	GetKind() string
}
