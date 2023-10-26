package v1alpha2

import (
	"strconv"

	"github.com/google/go-containerregistry/pkg/name"
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
	return b.Spec.ServiceAccountName
}

func (b *Build) BuilderSpec() corev1alpha1.BuildBuilderSpec {
	return b.Spec.Builder
}

func (b *Build) Services() Services {
	return b.Spec.Services
}

func (b *Build) CnbBindings() corev1alpha1.CNBBindings {
	return b.Spec.CNBBindings
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

func (b *Build) BuildReason() string {
	if b == nil {
		return ""
	}
	return b.GetAnnotations()[BuildReasonAnnotation]
}

func (b *Build) BuildChanges() string {
	if b == nil {
		return ""
	}
	return b.GetAnnotations()[BuildChangesAnnotation]
}

func (b *Build) PriorityClassName() string {
	if b == nil {
		return ""
	}
	return b.Spec.PriorityClassName
}

func (b *Build) ImageGeneration() int64 {
	if b == nil {
		return 0
	}
	generation, ok := b.Labels[ImageGenerationLabel]
	if !ok {
		return 0
	}
	atoi, err := strconv.Atoi(generation)
	if err != nil {
		return 0
	}

	return int64(atoi)
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

func (b *Build) CacheImage() string {
	if b == nil {
		return ""
	}
	if !b.IsSuccess() {
		return ""
	}

	return b.Status.LatestCacheImage
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
		(pod.Status.Phase == corev1.PodSucceeded || podCompletedWithActiveDeadline(pod))
}

func (b *Build) Finished() bool {
	return !b.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsUnknown()
}

func (b *Build) NotaryV1Config() *corev1alpha1.NotaryV1Config {
	if b.Spec.Notary == nil {
		return nil
	}
	return b.Spec.Notary.V1
}

func (b *Build) DefaultProcess() string {
	return b.Spec.DefaultProcess
}

var buildSteps = map[string]struct{}{
	PrepareContainerName:    {},
	AnalyzeContainerName:    {},
	DetectContainerName:     {},
	RestoreContainerName:    {},
	ExtendContainerName:     {},
	BuildContainerName:      {},
	ExportContainerName:     {},
	CompletionContainerName: {},
	RebaseContainerName:     {},
}

func BuildSteps() map[string]struct{} {
	return buildSteps
}

func IsBuildStep(step string) bool {
	_, found := buildSteps[step]
	return found
}

func (b *Build) rebasable(builderStack string) bool {
	return b.Spec.LastBuild != nil &&
		b.Annotations[BuildReasonAnnotation] == BuildReasonStack && b.Spec.LastBuild.StackId == builderStack
}

func (b *Build) builtWithStack(runImage string) bool {
	if b.Status.Stack.RunImage == "" {
		return false
	}

	lastBuildRunImageRef, err := name.ParseReference(b.Status.Stack.RunImage)
	if err != nil {
		return false
	}

	builderRunImageRef, err := name.ParseReference(runImage)
	if err != nil {
		return false
	}

	return lastBuildRunImageRef.Identifier() == builderRunImageRef.Identifier()
}

func (b *Build) additionalBuildNeeded() bool {
	_, ok := b.Annotations[BuildNeededAnnotation]
	return ok
}

func (b *Build) builderName() string {
	if b == nil {
		return ""
	}
	return b.GetAnnotations()[BuilderNameAnnotation]
}

func (b *Build) builderKind() string {
	if b == nil {
		return ""
	}
	return b.GetAnnotations()[BuilderKindAnnotation]
}

func podCompletedWithActiveDeadline(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "DeadlineExceeded" {

		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == CompletionContainerName &&
				status.State.Terminated != nil &&
				status.State.Terminated.ExitCode == 0 {
				return true
			}
		}
	}
	return false
}
