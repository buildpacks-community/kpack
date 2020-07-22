package duckbuilder

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type DuckBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DuckBuilderSpec              `json:"spec"`
	Status v1alpha1.CustomBuilderStatus `json:"status"`
}
type DuckBuilderSpec struct {
	ImagePullSecrets []v1.LocalObjectReference
}

func (b *DuckBuilder) Ready() bool {
	return b.Status.GetCondition(corev1alpha1.ConditionReady).IsTrue() &&
		(b.Generation == b.Status.ObservedGeneration)
}

func (b *DuckBuilder) BuildBuilderSpec() v1alpha1.BuildBuilderSpec {
	return v1alpha1.BuildBuilderSpec{
		Image:            b.Status.LatestImage,
		ImagePullSecrets: b.Spec.ImagePullSecrets,
	}
}

func (b *DuckBuilder) BuildpackMetadata() v1alpha1.BuildpackMetadataList {
	return b.Status.BuilderMetadata
}

func (b *DuckBuilder) RunImage() string {
	return b.Status.Stack.RunImage
}
