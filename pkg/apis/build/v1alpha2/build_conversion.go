package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (b *Build) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toBuild := to.(type) {
	case *v1alpha1.Build:
		toBuild.ObjectMeta = b.ObjectMeta
		b.Spec.convertTo(&toBuild.Spec)
		b.Status.convertTo(&toBuild.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toBuild)
	}

	return nil
}

func (b *Build) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromBuild := from.(type) {
	case *v1alpha1.Build:
		b.ObjectMeta = fromBuild.ObjectMeta
		b.Spec.convertFrom(&fromBuild.Spec)
		b.Status.convertFrom(&fromBuild.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromBuild)
	}
	return nil
}

func (bs *BuildSpec) convertTo(to *v1alpha1.BuildSpec) {
	to.Env = bs.Env
	to.Source = bs.Source
	to.Resources = bs.Resources
	to.Tags = bs.Tags
	if bs.Cache != nil && bs.Cache.Volume != nil {
		to.CacheName = bs.Cache.Volume.ClaimName
	}
	if bs.LastBuild != nil {
		to.LastBuild = &v1alpha1.LastBuild{
			Image:   bs.LastBuild.Image,
			StackId: bs.LastBuild.StackId,
		}
	}
	to.ServiceAccount = bs.ServiceAccountName
	to.Builder = bs.Builder
	to.Notary = bs.Notary
	to.Bindings = bs.CNBBindings
}

func (bs *BuildSpec) convertFrom(from *v1alpha1.BuildSpec) {
	bs.Env = from.Env
	bs.Source = from.Source
	if from.CacheName != "" {
		bs.Cache = &BuildCacheConfig{
			Volume: &BuildPersistentVolumeCache{
				ClaimName: from.CacheName,
			},
		}
	}
	bs.Resources = from.Resources
	bs.Tags = from.Tags
	if from.LastBuild != nil {
		bs.LastBuild = &LastBuild{
			Image:   from.LastBuild.Image,
			StackId: from.LastBuild.StackId,
		}
	}
	bs.ServiceAccountName = from.ServiceAccount
	bs.Builder = from.Builder
	bs.Notary = from.Notary
	bs.CNBBindings = from.Bindings
}

func (bs *BuildStatus) convertFrom(from *v1alpha1.BuildStatus) {
	bs.Status = from.Status
	bs.BuildMetadata = from.BuildMetadata
	bs.Stack = from.Stack
	bs.LatestImage = from.LatestImage
	bs.PodName = from.PodName
	bs.StepStates = from.StepStates
	bs.StepsCompleted = from.StepsCompleted
}

func (bs *BuildStatus) convertTo(to *v1alpha1.BuildStatus) {
	to.Status = bs.Status
	to.BuildMetadata = bs.BuildMetadata
	to.Stack = bs.Stack
	to.LatestImage = bs.LatestImage
	to.PodName = bs.PodName
	to.StepStates = bs.StepStates
	to.StepsCompleted = bs.StepsCompleted
}
