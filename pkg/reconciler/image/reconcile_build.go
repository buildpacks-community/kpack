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
	BuildFailedReason      = "BuildFailed"
	UpToDateReason         = "UpToDate"
)

func (c *Reconciler) reconcileBuild(ctx context.Context, image *buildapi.Image, latestBuild *buildapi.Build, sourceResolver *buildapi.SourceResolver, builder buildapi.BuilderResource, buildCacheName string) (buildapi.ImageStatus, error) {
	currentBuildNumber, err := buildCounter(latestBuild)
	if err != nil {
		return buildapi.ImageStatus{}, err
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
			return buildapi.ImageStatus{}, err
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
				Conditions: noScheduledBuild(result, builder, latestBuild, sourceResolver),
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
		return buildapi.ImageStatus{}, errors.Errorf("unexpected build needed condition %s", result.ConditionStatus)
	}
}

func noScheduledBuild(buildNeeded buildRequiredResult, builder buildapi.BuilderResource, build *buildapi.Build, sourceResolver *buildapi.SourceResolver) corev1alpha1.Conditions {
	if !builder.Ready() {
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionReady,
				Status:             corev1.ConditionFalse,
				Reason:             buildapi.BuilderNotReady,
				Message:            builderError(builder),
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
			builderCondition(builder),
		}
	}
	if buildNeeded.ConditionStatus == corev1.ConditionUnknown {
		message := "Build status unknown"
		if !sourceResolver.Ready() {
			message = fmt.Sprintf("SourceResolver %s is not ready", sourceResolver.GetName())
		}
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionReady,
				Status:             corev1.ConditionUnknown,
				Reason:             ResolverNotReadyReason,
				Message:            message,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
			builderCondition(builder),
		}
	}

	buildReason := UpToDateReason
	buildMessage := "Last build succeeded"

	if !build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue() {
		buildReason = BuildFailedReason
		buildMessage = "Last build did not succeed"
	}

	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             unknownStatusIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded)),
			Reason:             buildReason,
			Message:            defaultMessageIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded), buildMessage),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		builderCondition(builder),
	}

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
	errorMessage := fmt.Sprintf("Builder %s is not ready", builder.GetName())

	if message := builder.ConditionReadyMessage(); message != "" {
		errorMessage = fmt.Sprintf("%s: %s", errorMessage, message)
	}

	return errorMessage
}

func scheduledBuildCondition(build *buildapi.Build) corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionUnknown,
			Reason:             BuildRunningReason,
			Message:            fmt.Sprintf("%s is executing", build.Name),
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
	return corev1alpha1.Conditions{
		{
			Type:   corev1alpha1.ConditionReady,
			Status: corev1.ConditionUnknown,
			Reason: BuildRunningReason,
			Message: defaultMessageIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded),
				fmt.Sprintf("%s is executing", build.Name)),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		builderCondition(builder),
	}
}
