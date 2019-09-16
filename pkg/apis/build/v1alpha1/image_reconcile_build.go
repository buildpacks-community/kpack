package v1alpha1

import (
	"fmt"
	"strconv"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func (im *Image) ReconcileBuild(latestBuild *Build, resolver *SourceResolver, builder AbstractBuilder) (BuildApplier, error) {
	currentBuildNumber, err := buildCounter(latestBuild)
	if err != nil {
		return nil, err
	}
	latestImage := im.latestForImage(latestBuild)

	if reasons, needed := im.buildNeeded(latestBuild, resolver, builder); needed {
		nextBuildNumber := currentBuildNumber + 1
		return newBuild{
			previousBuild: latestBuild,
			build:         im.build(resolver, builder, reasons, nextBuildNumber),
			buildCounter:  nextBuildNumber,
			latestImage:   latestImage,
		}, nil
	}

	return upToDateBuild{
		build:        latestBuild,
		buildCounter: currentBuildNumber,
		latestImage:  latestImage,
		builder:      builder,
	}, nil
}

type BuildCreator interface {
	CreateBuild(*Build) (*Build, error)
}

type ReconciledBuild struct {
	Build        *Build
	BuildCounter int64
	LatestImage  string
	Conditions   duckv1alpha1.Conditions
}

type BuildApplier interface {
	Apply(creator BuildCreator) (ReconciledBuild, error)
}

type upToDateBuild struct {
	build        *Build
	buildCounter int64
	latestImage  string
	builder      AbstractBuilder
}

func (r upToDateBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	return ReconciledBuild{
		Build:        r.build,
		BuildCounter: r.buildCounter,
		LatestImage:  r.latestImage,
		Conditions:   r.conditions(),
	}, nil
}

func (r upToDateBuild) conditions() duckv1alpha1.Conditions {
	if r.build == nil || r.build.Status.GetCondition(duckv1alpha1.ConditionSucceeded) == nil {
		return duckv1alpha1.Conditions{
			{
				Type:   duckv1alpha1.ConditionReady,
				Status: corev1.ConditionUnknown,
			}, r.builderCondition(),
		}
	}

	condition := r.build.Status.GetCondition(duckv1alpha1.ConditionSucceeded)

	return duckv1alpha1.Conditions{
		{
			Type:   duckv1alpha1.ConditionReady,
			Status: condition.Status,
		}, r.builderCondition(),
	}
}

func (r upToDateBuild) builderCondition() duckv1alpha1.Condition {
	if !r.builder.Ready() {
		return duckv1alpha1.Condition{
			Type:    ConditionBuilderReady,
			Status:  corev1.ConditionFalse,
			Reason:  BuilderNotReady,
			Message: fmt.Sprintf("Builder %s is not ready", r.builder.GetName()),
		}
	}
	return duckv1alpha1.Condition{
		Type:   ConditionBuilderReady,
		Status: corev1.ConditionTrue,
	}
}

type newBuild struct {
	build         *Build
	buildCounter  int64
	latestImage   string
	previousBuild *Build
}

func (r newBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	build, err := creator.CreateBuild(r.build)
	return ReconciledBuild{
		Build:        build,
		BuildCounter: r.buildCounter,
		LatestImage:  r.latestImage,
		Conditions:   r.conditions(),
	}, err
}

func (r newBuild) conditions() duckv1alpha1.Conditions {
	return duckv1alpha1.Conditions{
		{
			Type:   duckv1alpha1.ConditionReady,
			Status: corev1.ConditionUnknown,
		},
		{
			Type:   ConditionBuilderReady,
			Status: corev1.ConditionTrue,
		},
	}
}

func buildCounter(build *Build) (int64, error) {
	if build == nil {
		return 0, nil
	}

	buildNumber := build.Labels[BuildNumberLabel]
	return strconv.ParseInt(buildNumber, 10, 64)
}
