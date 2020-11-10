package image

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

func buildNeeded(im *v1alpha2.Image, lastBuild *v1alpha2.Build, sourceResolver *v1alpha2.SourceResolver, builder v1alpha2.BuilderResource) ([]string, corev1.ConditionStatus) {
	if !sourceResolver.Ready() || !builder.Ready() {
		return nil, corev1.ConditionUnknown
	}

	if lastBuild == nil || im.Spec.Tag != lastBuild.Tag() {
		return []string{v1alpha2.BuildReasonConfig}, corev1.ConditionTrue
	}

	var reasons []string

	if sourceResolver.ConfigChanged(lastBuild) ||
		!equality.Semantic.DeepEqual(im.Env(), lastBuild.Spec.Env) ||
		!equality.Semantic.DeepEqual(im.Resources(), lastBuild.Spec.Resources) ||
		!equality.Semantic.DeepEqual(im.Services(), lastBuild.Spec.Services) {
		reasons = append(reasons, v1alpha2.BuildReasonConfig)
	}

	if sourceResolver.RevisionChanged(lastBuild) {
		reasons = append(reasons, v1alpha2.BuildReasonCommit)
	}

	if lastBuild.IsSuccess() {
		if !builtWithBuildpacks(lastBuild, builder.BuildpackMetadata()) {
			reasons = append(reasons, v1alpha2.BuildReasonBuildpack)
		}

		if !builtWithStack(lastBuild, builder.RunImage()) {
			reasons = append(reasons, v1alpha2.BuildReasonStack)
		}
	}

	if additionalBuildNeeded(lastBuild) {
		reasons = append(reasons, v1alpha2.BuildReasonTrigger)
	}

	if len(reasons) == 0 {
		return nil, corev1.ConditionFalse
	}

	return reasons, corev1.ConditionTrue
}

func builtWithBuildpacks(build *v1alpha2.Build, buildpacks v1alpha2.BuildpackMetadataList) bool {
	for _, bp := range build.Status.BuildMetadata {
		if !buildpacks.Include(bp) {
			return false
		}
	}

	return true
}

func builtWithStack(build *v1alpha2.Build, runImage string) bool {
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

func additionalBuildNeeded(build *v1alpha2.Build) bool {
	_, ok := build.Annotations[v1alpha2.BuildNeededAnnotation]
	return ok
}
