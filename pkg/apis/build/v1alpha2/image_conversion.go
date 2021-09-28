package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (i *Image) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toImage := to.(type) {
	case *v1alpha1.Image:
		toImage.ObjectMeta = i.ObjectMeta
		i.Spec.convertTo(&toImage.Spec)
		i.Status.convertTo(&toImage.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toImage)
	}
	return nil
}

func (i *Image) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromImage := from.(type) {
	case *v1alpha1.Image:
		i.ObjectMeta = fromImage.ObjectMeta
		i.Spec.convertFrom(&fromImage.Spec)
		i.Status.convertFrom(&fromImage.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromImage)
	}

	return nil
}

func (is *ImageSpec) convertTo(to *v1alpha1.ImageSpec) {
	to.Tag = is.Tag
	to.Builder = is.Builder
	to.ServiceAccount = is.ServiceAccountName
	if is.Cache != nil && is.Cache.Volume != nil {
		to.CacheSize = is.Cache.Volume.Size
	}
	to.FailedBuildHistoryLimit = is.FailedBuildHistoryLimit
	to.SuccessBuildHistoryLimit = is.SuccessBuildHistoryLimit
	to.ImageTaggingStrategy = is.ImageTaggingStrategy
	to.Source = is.Source
	to.Notary = is.Notary

	if is.Build != nil {
		to.Build = &v1alpha1.ImageBuild{}
		to.Build.Env = is.Build.Env
		to.Build.Resources = is.Build.Resources
		to.Build.Bindings = is.Build.CNBBindings
	}
}

func (is *ImageSpec) convertFrom(from *v1alpha1.ImageSpec) {
	is.Tag = from.Tag
	is.Builder = from.Builder
	is.ServiceAccountName = from.ServiceAccount
	is.Source = from.Source
	if from.CacheSize != nil {
		is.Cache = &ImageCacheConfig{
			Volume: &ImagePersistentVolumeCache{
				Size: from.CacheSize,
			},
		}
	}
	is.FailedBuildHistoryLimit = from.FailedBuildHistoryLimit
	is.SuccessBuildHistoryLimit = from.SuccessBuildHistoryLimit
	is.ImageTaggingStrategy = from.ImageTaggingStrategy
	is.Notary = from.Notary

	if from.Build != nil {
		is.Build = &ImageBuild{}
		is.Build.Env = from.Build.Env
		is.Build.Resources = from.Build.Resources
		is.Build.CNBBindings = from.Build.Bindings
	}
}

func (is *ImageStatus) convertFrom(from *v1alpha1.ImageStatus) {
	is.LatestBuildImageGeneration = from.LatestBuildImageGeneration
	is.BuildCounter = from.BuildCounter
	is.BuildCacheName = from.BuildCacheName
	is.LatestBuildReason = from.LatestBuildReason
	is.LatestBuildRef = from.LatestBuildRef
	is.LatestImage = from.LatestImage
	is.LatestStack = from.LatestStack
	is.Status = from.Status
}

func (is *ImageStatus) convertTo(to *v1alpha1.ImageStatus) {
	to.LatestBuildImageGeneration = is.LatestBuildImageGeneration
	to.BuildCounter = is.BuildCounter
	to.BuildCacheName = is.BuildCacheName
	to.LatestBuildReason = is.LatestBuildReason
	to.LatestBuildRef = is.LatestBuildRef
	to.LatestImage = is.LatestImage
	to.LatestStack = is.LatestStack
	to.Status = is.Status
}
