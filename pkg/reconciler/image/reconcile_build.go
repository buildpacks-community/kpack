package image

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func (c *Reconciler) reconcileBuild(image *v1alpha2.Image, latestBuild *v1alpha2.Build, sourceResolver *v1alpha1.SourceResolver, builder v1alpha1.BuilderResource, buildCacheName string) (v1alpha2.ImageStatus, error) {
	currentBuildNumber, err := buildCounter(latestBuild)
	if err != nil {
		return v1alpha2.ImageStatus{}, err
	}

	reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
	switch needed {
	case corev1.ConditionTrue:
		nextBuildNumber := currentBuildNumber + 1
		build := image.Build(sourceResolver, builder, latestBuild, reasons, nextBuildNumber)

		build, err := c.Client.KpackV1alpha2().Builds(build.Namespace).Create(build)
		if err != nil {
			return v1alpha2.ImageStatus{}, err
		}
		return v1alpha2.ImageStatus{
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
		return v1alpha2.ImageStatus{
			Status: corev1alpha1.Status{
				Conditions: noScheduledBuild(needed, builder, latestBuild),
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
		return v1alpha2.ImageStatus{}, errors.Errorf("unexpected build needed condition %s", needed)
	}

}

func noScheduledBuild(buildNeeded corev1.ConditionStatus, builder v1alpha1.BuilderResource, build *v1alpha2.Build) corev1alpha1.Conditions {
	if buildNeeded == corev1.ConditionUnknown {
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionReady,
				Status:             corev1.ConditionUnknown,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
			builderCondition(builder),
		}
	}

	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             unknownIfNil(build.Status.GetCondition(corev1alpha1.ConditionSucceeded)),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		builderCondition(builder),
	}

}

func unknownIfNil(condition *corev1alpha1.Condition) corev1.ConditionStatus {
	if condition == nil {
		return corev1.ConditionUnknown
	}
	return condition.Status
}

func builderCondition(builder v1alpha1.BuilderResource) corev1alpha1.Condition {
	if !builder.Ready() {
		return corev1alpha1.Condition{
			Type:               v1alpha2.ConditionBuilderReady,
			Status:             corev1.ConditionFalse,
			Reason:             v1alpha2.BuilderNotReady,
			Message:            fmt.Sprintf("Builder %s is not ready", builder.GetName()),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		}
	}
	return corev1alpha1.Condition{
		Type:               v1alpha2.ConditionBuilderReady,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
	}
}

func scheduledBuildCondition(build *v1alpha2.Build) corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			Message:            fmt.Sprintf("%s is executing", build.Name),
		},
		{
			Type:               v1alpha2.ConditionBuilderReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
	}
}

func buildCounter(build *v1alpha2.Build) (int64, error) {
	if build == nil {
		return 0, nil
	}

	buildNumber := build.Labels[v1alpha2.BuildNumberLabel]
	return strconv.ParseInt(buildNumber, 10, 64)
}
