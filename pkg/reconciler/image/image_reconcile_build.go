package image

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

func reconcileBuild(image *v1alpha1.Image, latestBuild *v1alpha1.Build, resolver *v1alpha1.SourceResolver, builder v1alpha1.BuilderResource) (buildApplier, error) {
	currentBuildNumber, err := buildCounter(latestBuild)
	if err != nil {
		return nil, err
	}

	if reasons, needed := image.BuildNeeded(latestBuild, resolver, builder); needed {
		nextBuildNumber := currentBuildNumber + 1
		return newBuild{
			previousBuild: latestBuild,
			build:         image.Build(resolver, builder, latestBuild, reasons, nextBuildNumber),
			buildCounter:  nextBuildNumber,
			latestImage:   image.LatestForImage(latestBuild),
		}, nil
	}

	return upToDateBuild{
		build:        latestBuild,
		buildCounter: currentBuildNumber,
		latestImage:  image.LatestForImage(latestBuild),
		builder:      builder,
	}, nil
}

type ReconciledBuild struct {
	Build        *v1alpha1.Build
	BuildCounter int64
	LatestImage  string
	Conditions   corev1alpha1.Conditions
}

type buildApplier interface {
	Apply(versioned.Interface) (ReconciledBuild, error)
}

type upToDateBuild struct {
	build        *v1alpha1.Build
	buildCounter int64
	latestImage  string
	builder      v1alpha1.BuilderResource
}

func (r upToDateBuild) Apply(_ versioned.Interface) (ReconciledBuild, error) {
	return ReconciledBuild{
		Build:        r.build,
		BuildCounter: r.buildCounter,
		LatestImage:  r.latestImage,
		Conditions:   r.conditions(),
	}, nil
}

func (r upToDateBuild) conditions() corev1alpha1.Conditions {
	if r.build == nil || r.build.Status.GetCondition(corev1alpha1.ConditionSucceeded) == nil {
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionReady,
				Status:             corev1.ConditionUnknown,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			}, r.builderCondition(),
		}
	}

	condition := r.build.Status.GetCondition(corev1alpha1.ConditionSucceeded)

	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             condition.Status,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		}, r.builderCondition(),
	}
}

func (r upToDateBuild) builderCondition() corev1alpha1.Condition {
	if !r.builder.Ready() {
		return corev1alpha1.Condition{
			Type:    v1alpha1.ConditionBuilderReady,
			Status:  corev1.ConditionFalse,
			Reason:  v1alpha1.BuilderNotReady,
			Message: fmt.Sprintf("Builder %s is not ready", r.builder.GetName()),
		}
	}
	return corev1alpha1.Condition{
		Type:   v1alpha1.ConditionBuilderReady,
		Status: corev1.ConditionTrue,
	}
}

type newBuild struct {
	build         *v1alpha1.Build
	buildCounter  int64
	latestImage   string
	previousBuild *v1alpha1.Build
}

func (r newBuild) Apply(client versioned.Interface) (ReconciledBuild, error) {
	build, err := client.BuildV1alpha1().Builds(r.build.Namespace).Create(r.build)
	return ReconciledBuild{
		Build:        build,
		BuildCounter: r.buildCounter,
		LatestImage:  r.latestImage,
		Conditions:   r.conditions(),
	}, err
}

func (r newBuild) conditions() corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionUnknown,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
		{
			Type:               v1alpha1.ConditionBuilderReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
	}
}

func buildCounter(build *v1alpha1.Build) (int64, error) {
	if build == nil {
		return 0, nil
	}

	buildNumber := build.Labels[v1alpha1.BuildNumberLabel]
	return strconv.ParseInt(buildNumber, 10, 64)
}
