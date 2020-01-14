package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/kmeta"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func (*Build) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Build")
}

func (b *Build) Tag() string {
	return b.Spec.Tags[0]
}

func (b *Build) ServiceAccount() string {
	return b.Spec.ServiceAccount
}

func (b *Build) BuilderSpec() BuildBuilderSpec {
	return b.Spec.Builder
}

func (b *Build) IsRunning() bool {
	if b == nil {
		return false
	}

	return b.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsUnknown()
}

func (b *Build) BuildRef() string {
	if b == nil {
		return ""
	}

	return b.GetName()
}

func (b *Build) Stack() string {
	if b == nil {
		return ""
	}
	if !b.IsSuccess() {
		return ""
	}
	return b.Status.Stack.ID
}

func (b *Build) BuiltImage() string {
	if b == nil {
		return ""
	}
	if !b.IsSuccess() {
		return ""
	}

	return b.Status.LatestImage
}

func (b *Build) IsSuccess() bool {
	if b == nil {
		return false
	}
	return b.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue()
}

func (b *Build) IsFailure() bool {
	if b == nil {
		return false
	}
	return b.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsFalse()
}

func (b *Build) PodName() string {
	return kmeta.ChildName(b.Name, "-build-pod")
}

func (b *Build) MetadataReady(pod *corev1.Pod) bool {
	return !b.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue() &&
		pod.Status.Phase == "Succeeded"
}

func (b *Build) Finished() bool {
	return !b.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsUnknown()
}

func (b *Build) rebasable(builderStack string) bool {
	return b.Spec.LastBuild != nil &&
		b.Annotations[BuildReasonAnnotation] == BuildReasonStack && b.Spec.LastBuild.StackId == builderStack
}
