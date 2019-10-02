package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuilderResource interface {
	metav1.ObjectMetaAccessor
	BuildBuilderSpec() BuildBuilderSpec
	Image() string
	ImagePullSecrets() []v1.LocalObjectReference
	Ready() bool
	BuildpackMetadata() BuildpackMetadataList
	RunImage() string
}
