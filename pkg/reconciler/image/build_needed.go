package image

import (
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func buildNeeded(im *v1alpha1.Image, lastBuild *v1alpha1.Build, sourceResolver *v1alpha1.SourceResolver, builder v1alpha1.BuilderResource) ([]string, corev1.ConditionStatus) {
	if !sourceResolver.Ready() || !builder.Ready() {
		return nil, corev1.ConditionUnknown
	}

	if lastBuild == nil || im.Spec.Tag != lastBuild.Tag() {
		return []string{v1alpha1.BuildReasonConfig}, corev1.ConditionTrue
	}

	var reasons []string

	if sourceResolver.ConfigChanged(lastBuild) ||
		!equality.Semantic.DeepEqual(im.Env(), lastBuild.Spec.Env) ||
		!equality.Semantic.DeepEqual(im.Resources(), lastBuild.Spec.Resources) {
		reasons = append(reasons, v1alpha1.BuildReasonConfig)
	}

	if sourceResolver.RevisionChanged(lastBuild) {
		reasons = append(reasons, v1alpha1.BuildReasonCommit)
	}

	if lastBuild.IsSuccess() {
		if !builtWithBuildpacks(lastBuild, builder.BuildpackMetadata()) {
			reasons = append(reasons, v1alpha1.BuildReasonBuildpack)
		}

		if !builtWithStack(lastBuild, builder.RunImage()) {
			reasons = append(reasons, v1alpha1.BuildReasonStack)
		}
	}

	if additionalBuildNeeded(lastBuild) {
		reasons = append(reasons, v1alpha1.BuildReasonTrigger)
	}

	if len(reasons) == 0 {
		return nil, corev1.ConditionFalse
	}

	return reasons, corev1.ConditionTrue
}

func builtWithBuildpacks(build *v1alpha1.Build, buildpacks v1alpha1.BuildpackMetadataList) bool {
	for _, bp := range build.Status.BuildMetadata {
		if !buildpacks.Include(bp) {
			return false
		}
	}

	return true
}

func builtWithStack(build *v1alpha1.Build, runImage string) bool {
	if build.Status.Stack.RunImage == "" {
		return false
	}

	lastBuildRunImageRef, err := name.ParseReference(build.Status.Stack.RunImage)
	if err != nil {
		return false
	}

	builderRunImageRef, err := name.ParseReference(runImage)
	if err != nil {
		return false
	}

	return lastBuildRunImageRef.Identifier() == builderRunImageRef.Identifier()
}

func additionalBuildNeeded(build *v1alpha1.Build) bool {
	_, ok := build.Annotations[v1alpha1.BuildNeededAnnotation]
	return ok
}
