package image

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	BuildRunningReason     = "BuildRunning"
	ResolverNotReadyReason = "ResolverNotReady"
	UnknownStateReason     = "UnknownState"
	BuildFailedReason      = "BuildFailed"
	UpToDateReason         = "UpToDate"
)

func (c *Reconciler) reconcileBuild(ctx context.Context, image *buildapi.Image, latestBuild *buildapi.Build, sourceResolver *buildapi.SourceResolver, builder buildapi.BuilderResource, buildCacheName string) (buildapi.ImageStatus, error) {
	currentBuildNumber, err := buildCounter(latestBuild)
	if err != nil {
		return buildapi.ImageStatus{}, errors.Wrap(err, "error parsing the image build number")
	}

	result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
	if err != nil {
		return buildapi.ImageStatus{}, errors.Wrap(err, "error determining if an image build is needed")
	}
	priorityClass := ""
	if c.EnablePriorityClasses {
		priorityClass = result.PriorityClass
	}
	switch result.ConditionStatus {
	case corev1.ConditionTrue:
		nextBuildNumber := currentBuildNumber + 1
		build := image.Build(sourceResolver, builder, latestBuild, result.ReasonsStr, result.ChangesStr, nextBuildNumber, priorityClass)
		build, err = c.Client.KpackV1alpha2().Builds(build.Namespace).Create(ctx, build, metav1.CreateOptions{})
		if err != nil {
			return buildapi.ImageStatus{}, errors.WithMessage(err, fmt.Sprintf("error creating build '%s' in namespace '%s'", build.Name, build.Namespace))
		}

		return buildapi.ImageStatus{
			Status: corev1alpha1.Status{
				Conditions: scheduledBuildCondition(build),
			},
			BuildCounter:               nextBuildNumber,
			BuildCacheName:             buildCacheName,
			LatestBuildRef:             build.BuildRef(),
			LatestBuildReason:          build.BuildReason(),
			LatestImage:                image.LatestForImage(latestBuild),
			LatestStack:                build.Stack(),
			LatestBuildImageGeneration: build.ImageGeneration(),
		}, nil
	case corev1.ConditionUnknown:
		fallthrough
	case corev1.ConditionFalse:
		return buildapi.ImageStatus{
			Status: corev1alpha1.Status{
				Conditions: noScheduledBuild(result.ConditionStatus, builder, latestBuild, sourceResolver),
			},
			LatestBuildRef:             latestBuild.BuildRef(),
			LatestBuildReason:          latestBuild.BuildReason(),
			LatestBuildImageGeneration: latestBuild.ImageGeneration(),
			LatestImage:                image.LatestForImage(latestBuild),
			LatestStack:                latestBuild.Stack(),
			BuildCounter:               currentBuildNumber,
			BuildCacheName:             buildCacheName,
		}, nil
	default:
		return buildapi.ImageStatus{}, errors.Errorf("Error: unexpected build needed condition %s", result.ConditionStatus)
	}
}

func noScheduledBuild(buildNeeded corev1.ConditionStatus, builder buildapi.BuilderResource, build *buildapi.Build, sourceResolver *buildapi.SourceResolver) corev1alpha1.Conditions {
	ready := corev1alpha1.Condition{
		Type:               corev1alpha1.ConditionReady,
		Status:             corev1.ConditionUnknown,
		LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
	}

	switch {
	case !builder.Ready():
		ready.Status = corev1.ConditionFalse
		ready.Reason = buildapi.BuilderNotReady
		ready.Message = builderError(builder)
	case buildNeeded == corev1.ConditionUnknown && !sourceResolver.Ready():
		ready.Status = corev1.ConditionUnknown
		ready.Reason = ResolverNotReadyReason
		ready.Message = fmt.Sprintf("Error: SourceResolver '%s' is not ready", sourceResolver.GetName())
	case buildNeeded == corev1.ConditionUnknown && sourceResolver.Ready():
		ready.Status = corev1.ConditionUnknown
		ready.Reason = UnknownStateReason
		ready.Message = "Error: Build status unknown"
	case build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue():
		ready.Status = corev1.ConditionTrue
		ready.Reason = UpToDateReason
		ready.Message = defaultMessageIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded), "Last build succeeded")
	default:
		ready.Status = unknownStatusIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded))
		ready.Reason = BuildFailedReason
		ready.Message = fmt.Sprintf("Error: Build '%s' in namespace '%s' failed: %s", build.Name, build.Namespace, defaultMessageIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded), "unknown error"))
	}

	return corev1alpha1.Conditions{ready, builderCondition(builder)}

}

func unknownStatusIfNil(condition *corev1alpha1.Condition) corev1.ConditionStatus {
	if condition == nil {
		return corev1.ConditionUnknown
	}
	return condition.Status
}

// Copies the message from the specified condition, or fills in a default message if nil.
// We should always have a message for non-successful conditions, as that conveys
// information to the user about what is expected.
func defaultMessageIfNil(condition *corev1alpha1.Condition, defaultMessage string) string {
	if condition == nil {
		return defaultMessage
	}
	return condition.Message
}

func builderCondition(builder buildapi.BuilderResource) corev1alpha1.Condition {
	if !builder.Ready() {
		return corev1alpha1.Condition{
			Type:               buildapi.ConditionBuilderReady,
			Status:             corev1.ConditionFalse,
			Reason:             buildapi.BuilderNotReady,
			Message:            builderError(builder),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		}
	}
	return corev1alpha1.Condition{
		Type:               buildapi.ConditionBuilderReady,
		Status:             corev1.ConditionTrue,
		Reason:             buildapi.BuilderReady,
		LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
	}
}

func builderError(builder buildapi.BuilderResource) string {
	errorMessage := fmt.Sprintf("Error: Builder '%s' is not ready in namespace '%s'", builder.GetName(), builder.GetNamespace())

	if message := builder.ConditionReadyMessage(); message != "" {
		errorMessage = fmt.Sprintf("%s; Message: %s", errorMessage, message)
	}

	return errorMessage
}

func scheduledBuildCondition(build *buildapi.Build) corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionUnknown,
			Reason:             BuildRunningReason,
			Message:            fmt.Sprintf("Build '%s' is executing", build.Name),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		{
			Type:               buildapi.ConditionBuilderReady,
			Status:             corev1.ConditionTrue,
			Reason:             buildapi.BuilderReady,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
	}
}

func buildCounter(build *buildapi.Build) (int64, error) {
	if build == nil {
		return 0, nil
	}

	buildNumber := build.Labels[buildapi.BuildNumberLabel]
	return strconv.ParseInt(buildNumber, 10, 64)
}

func buildRunningCondition(build *buildapi.Build, builder buildapi.BuilderResource) corev1alpha1.Conditions {
	message := defaultMessageIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded),
		fmt.Sprintf("Build '%s' is executing", build.Name))
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionUnknown,
			Reason:             BuildRunningReason,
			Message:            message,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		builderCondition(builder),
	}
}
