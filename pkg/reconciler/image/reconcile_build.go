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
		return buildapi.ImageStatus{}, errors.Errorf("unexpected build needed condition %s", result.ConditionStatus)
	}
}

func noScheduledBuild(buildNeeded corev1.ConditionStatus, builder buildapi.BuilderResource, build *buildapi.Build, sourceResolver *buildapi.SourceResolver) corev1alpha1.Conditions {
	if buildNeeded == corev1.ConditionUnknown {
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionReady,
				Status:             corev1.ConditionUnknown,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
			builderCondition(builder),
			sourceResolverCondition(sourceResolver),
		}
	}

	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             unknownStatusIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded)),
			Message:            emptyMessageIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded)),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		builderCondition(builder),
		sourceResolverCondition(sourceResolver),
	}

}

func unknownStatusIfNil(condition *corev1alpha1.Condition) corev1.ConditionStatus {
	if condition == nil {
		return corev1.ConditionUnknown
	}
	return condition.Status
}

func emptyMessageIfNil(condition *corev1alpha1.Condition) string {
	if condition == nil {
		return ""
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

func sourceResolverCondition(sourceResolver *buildapi.SourceResolver) corev1alpha1.Condition {
	switch unknownStatusIfNil(sourceResolver.Status.GetCondition(corev1alpha1.ConditionReady)) {
	case corev1.ConditionFalse:
		return corev1alpha1.Condition{
			Type:               buildapi.ConditionSourceResolverReady,
			Status:             corev1.ConditionFalse,
			Reason:             buildapi.SourceResolverNotReady,
			Message:            sourceResolverError(sourceResolver),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		}
	case corev1.ConditionTrue:
		return corev1alpha1.Condition{
			Type:               buildapi.ConditionSourceResolverReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		}
	default:
		return corev1alpha1.Condition{
			Type:               buildapi.ConditionSourceResolverReady,
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		}
	}
}

func sourceResolverError(sourceResolver *buildapi.SourceResolver) string {
	errorMessage := fmt.Sprintf("SourceResolver %s is not ready", sourceResolver.GetName())

	if message := emptyMessageIfNil(sourceResolver.Status.GetCondition(corev1alpha1.ConditionReady)); message != "" {
		errorMessage = fmt.Sprintf("%s: %s", errorMessage, message)
	}

	return errorMessage
}

func scheduledBuildCondition(build *buildapi.Build) corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			Message:            fmt.Sprintf("%s is executing", build.Name),
		},
		{
			Type:               buildapi.ConditionBuilderReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		{
			Type:               buildapi.ConditionSourceResolverReady,
			Status:             corev1.ConditionTrue,
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
