package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
)

func (bi *BuildBuilderSpec) getBuilderSecretVolume() corev1.Volume {
	if len(bi.ImagePullSecrets) > 0 {
		return corev1.Volume{
			Name: builderPullSecretsDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: bi.ImagePullSecrets[0].Name,
				},
			},
		}
	} else {
		return corev1.Volume{
			Name: builderPullSecretsDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
}

func (*Build) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Build")
}

func (b *Build) Tag() string {
	return b.Spec.Tags[0]
}

func (b *Build) IsRunning() bool {
	if b == nil {
		return false
	}

	return b.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsUnknown()
}

func (b *Build) BuildRef() string {
	if b == nil {
		return ""
	}

	return b.GetName()
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
	return b.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue()
}

func (b *Build) IsFailure() bool {
	if b == nil {
		return false
	}
	return b.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsFalse()
}

func (b *Build) PodName() string {
	return kmeta.ChildName(b.Name, "-build-pod")
}

func (b *Build) MetadataReady(pod *corev1.Pod) bool {
	return !b.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() &&
		pod.Status.Phase == "Succeeded"
}

func (b *Build) Finished() bool {
	return !b.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsUnknown()
}

func (b *Build) BuildEnvVars() []corev1.EnvVar {
	return b.Spec.Source.Source().BuildEnvVars()
}

func (b *Build) ImagePullSecretsVolume() corev1.Volume {
	return b.Spec.Source.Source().ImagePullSecretsVolume()
}

func (b *Build) Rebasable() bool {
	return b.Annotations[BuildReasonAnnotation] == BuildReasonStack
}
