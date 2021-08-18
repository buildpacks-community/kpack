package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (i *Image) ConvertTo(_ context.Context, to apis.Convertible) error {
	toImage := to.(*v1alpha1.Image)
	toImage.ObjectMeta = i.ObjectMeta
	i.Spec.convertTo(&toImage.Spec)
	i.Status.convertTo(&toImage.Status)

	return nil
}

func (i *Image) ConvertFrom(_ context.Context, from apis.Convertible) error {
	fromImage := from.(*v1alpha1.Image)
	i.ObjectMeta = fromImage.ObjectMeta
	i.Spec.convertFrom(&fromImage.Spec)
	i.Status.convertFrom(&fromImage.Status)

	return nil
}

func (is *ImageSpec) convertTo(to *v1alpha1.ImageSpec) {
	to.Tag = is.Tag
	to.Builder = is.Builder
	to.ServiceAccount = is.ServiceAccount
	if is.Cache.Volume != nil {
		to.CacheSize = is.Cache.Volume.Size
	}
	to.FailedBuildHistoryLimit = is.FailedBuildHistoryLimit
	to.SuccessBuildHistoryLimit = is.SuccessBuildHistoryLimit
	to.ImageTaggingStrategy = is.ImageTaggingStrategy
	to.Source = is.Source
	to.Build = is.Build
	to.Notary = is.Notary
}

func (is *ImageSpec) convertFrom(from *v1alpha1.ImageSpec) {
	is.Tag = from.Tag
	is.Builder = from.Builder
	is.ServiceAccount = from.ServiceAccount
	is.Source = from.Source
	is.Cache = &ImageCacheConfig{
		Volume: &ImagePersistentVolumeCache{
			Size: from.CacheSize,
		},
	}
	is.FailedBuildHistoryLimit = from.FailedBuildHistoryLimit
	is.SuccessBuildHistoryLimit = from.SuccessBuildHistoryLimit
	is.ImageTaggingStrategy = from.ImageTaggingStrategy
	is.Build = from.Build
	is.Notary = from.Notary
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
