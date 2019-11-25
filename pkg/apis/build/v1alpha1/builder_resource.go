package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BuilderResource interface {
	metav1.ObjectMetaAccessor
	BuildBuilderSpec() BuildBuilderSpec
	Ready() bool
	BuildpackMetadata() BuildpackMetadataList
	RunImage() string
}
