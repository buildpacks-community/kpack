package duckbuilder

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type DuckBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DuckBuilderSpec        `json:"spec"`
	Status buildapi.BuilderStatus `json:"status"`
}

func (b *DuckBuilder) GetName() string {
	return b.Name
}

func (b *DuckBuilder) GetKind() string {
	return b.Kind
}

type DuckBuilderSpec struct {
	ImagePullSecrets []v1.LocalObjectReference
}

func (b *DuckBuilder) Ready() bool {
	return b.Status.GetCondition(corev1alpha1.ConditionReady).IsTrue() &&
		(b.Generation == b.Status.ObservedGeneration)
}

func (b *DuckBuilder) BuildBuilderSpec() corev1alpha1.BuildBuilderSpec {
	return corev1alpha1.BuildBuilderSpec{
		Image:            b.Status.LatestImage,
		ImagePullSecrets: b.Spec.ImagePullSecrets,
	}
}

func (b *DuckBuilder) BuildpackMetadata() corev1alpha1.BuildpackMetadataList {
	return b.Status.BuilderMetadata
}

func (b *DuckBuilder) RunImage() string {
	return b.Status.Stack.RunImage
}
