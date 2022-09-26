package v1alpha2

import (
	"context"
	"encoding/json"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const (
	servicesConversionAnnotation         = "kpack.io/services"
	storageClassNameConversionAnnotation = "kpack.io/cache.volume.storageClassName"
)

func (i *Image) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toImage := to.(type) {
	case *v1alpha1.Image:
		i.ObjectMeta.DeepCopyInto(&toImage.ObjectMeta)
		if toImage.Annotations == nil {
			toImage.Annotations = map[string]string{}
		}
		i.Spec.convertTo(&toImage.Spec)
		i.Status.convertTo(&toImage.Status)
		if build := i.Spec.Build; build != nil {
			if len(build.Services) > 0 {
				bytes, err := json.Marshal(build.Services)
				if err != nil {
					return err
				}
				toImage.Annotations[servicesConversionAnnotation] = string(bytes)
			}
		}
		if i.Spec.Cache != nil && i.Spec.Cache.Volume != nil && i.Spec.Cache.Volume.StorageClassName != "" {
			toImage.Annotations[storageClassNameConversionAnnotation] = i.Spec.Cache.Volume.StorageClassName
		}
	default:
		return fmt.Errorf("unknown version, got: %T", toImage)
	}
	return nil
}

func (i *Image) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromImage := from.(type) {
	case *v1alpha1.Image:
		fromImage.ObjectMeta.DeepCopyInto(&i.ObjectMeta)
		i.Spec.convertFrom(&fromImage.Spec)
		i.Status.convertFrom(&fromImage.Status)
		if servicesJson, ok := i.Annotations[servicesConversionAnnotation]; ok {
			var services Services
			err := json.Unmarshal([]byte(servicesJson), &services)
			if err != nil {
				return err
			}

			i.Spec.Build.Services = services
			delete(i.Annotations, servicesConversionAnnotation)
		}
		if storageClassName, ok := i.Annotations[storageClassNameConversionAnnotation]; ok {
			if i.Spec.Cache == nil {
				i.Spec.Cache = &ImageCacheConfig{}
			}
			if i.Spec.Cache.Volume == nil {
				i.Spec.Cache.Volume = &ImagePersistentVolumeCache{}
			}
			i.Spec.Cache.Volume.StorageClassName = storageClassName
			delete(i.Annotations, storageClassNameConversionAnnotation)
		}
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
