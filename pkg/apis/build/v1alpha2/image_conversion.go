package v1alpha2

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const (
	servicesConversionAnnotation              = "kpack.io/services"
	tolerationsConversionAnnotation           = "kpack.io/tolerations"
	nodeSelectorConversionAnnotation          = "kpack.io/nodeSelector"
	affinityConversionAnnotation              = "kpack.io/affinity"
	runtimeClassNameConversionAnnotation      = "kpack.io/runtimeClassName"
	schedulerNameConversionAnnotation         = "kpack.io/schedulerName"
	buildTimeoutConversionAnnotation          = "kpack.io/buildTimeout"
	storageClassNameConversionAnnotation      = "kpack.io/cache.volume.storageClassName"
	registryTagConversionAnnotation           = "kpack.io/cache.registry.tag"
	projectDescriptorPathConversionAnnotation = "kpack.io/projectDescriptorPath"
	cosignAnnotationConversionAnnotation      = "kpack.io/cosignAnnotation"
	defaultProcessConversionAnnotation        = "kpack.io/defaultProcess"
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
		if err := i.Spec.convertToAnnotations(toImage.Annotations); err != nil {
			return err
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
		i.convertFromAnnotations(&fromImage.Annotations)
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

func (i *Image)convertFromAnnotations(fromAnnotations *map[string]string) error {
	is := &i.Spec
	ia := i.Annotations
	if servicesJson, ok := (*fromAnnotations)[servicesConversionAnnotation]; ok {
		if is.Build == nil {
			is.Build = &ImageBuild{}
		}
		var services Services
		if err := json.Unmarshal([]byte(servicesJson), &services); err != nil {
			return err
		}
		is.Build.Services = services
		delete(ia, servicesConversionAnnotation)
	}
	if tolerationsJson, ok := (*fromAnnotations)[tolerationsConversionAnnotation]; ok {
		if is.Build == nil {
			is.Build = &ImageBuild{}
		}
		var tolerations []corev1.Toleration
		if err := json.Unmarshal([]byte(tolerationsJson), &tolerations); err != nil {
			return err
		}
		is.Build.Tolerations = tolerations
		delete(ia, tolerationsConversionAnnotation)
	}
	if nodeSelectorJson, ok := (*fromAnnotations)[nodeSelectorConversionAnnotation]; ok {
		if is.Build == nil {
			is.Build = &ImageBuild{}
		}
		var nodeSelector map[string]string
		if err := json.Unmarshal([]byte(nodeSelectorJson), &nodeSelector); err != nil {
			return err
		}
		is.Build.NodeSelector = nodeSelector
		delete(ia, nodeSelectorConversionAnnotation)
	}
	if affinityJson, ok := (*fromAnnotations)[affinityConversionAnnotation]; ok {
		if is.Build == nil {
			is.Build = &ImageBuild{}
		}
		var affinity *corev1.Affinity
		if err := json.Unmarshal([]byte(affinityJson), &affinity); err != nil {
			return err
		}
		is.Build.Affinity = affinity
		delete(ia, affinityConversionAnnotation)
	}
	if runtimeClassName, ok := (*fromAnnotations)[runtimeClassNameConversionAnnotation]; ok {
		if is.Build == nil {
			is.Build = &ImageBuild{}
		}
		is.Build.RuntimeClassName = &runtimeClassName
		delete(ia, runtimeClassNameConversionAnnotation)
	}
	if schedulerName, ok := (*fromAnnotations)[schedulerNameConversionAnnotation]; ok {
		if is.Build == nil {
			is.Build = &ImageBuild{}
		}
		is.Build.SchedulerName = schedulerName
		delete(ia, schedulerNameConversionAnnotation)
	}
	if buildTimeout, ok := (*fromAnnotations)[buildTimeoutConversionAnnotation]; ok {
		if is.Build == nil {
			is.Build = &ImageBuild{}
		}
		temp , err := strconv.ParseInt(buildTimeout, 10, 64)
		if err != nil {
			return err
		}
		is.Build.BuildTimeout = &temp
		delete(ia, buildTimeoutConversionAnnotation)
	}
	if storageClassName, ok := (*fromAnnotations)[storageClassNameConversionAnnotation]; ok {
		if is.Cache == nil {
			is.Cache = &ImageCacheConfig{}
		}
		if is.Cache.Volume == nil {
			is.Cache.Volume = &ImagePersistentVolumeCache{}
		}
		is.Cache.Volume.StorageClassName = storageClassName
		delete(ia, storageClassNameConversionAnnotation)
	}
	if registryTag, ok := (*fromAnnotations)[registryTagConversionAnnotation]; ok {
		if is.Cache == nil {
			is.Cache = &ImageCacheConfig{}
		}
		if is.Cache.Registry == nil {
			is.Cache.Registry = &RegistryCache{}
		}
		is.Cache.Registry.Tag = registryTag
		delete(ia, registryTagConversionAnnotation)
	}
	if projectDescriptorPath, ok := (*fromAnnotations)[projectDescriptorPathConversionAnnotation]; ok {
		is.ProjectDescriptorPath = projectDescriptorPath
		delete(ia, projectDescriptorPathConversionAnnotation)
	}
	if cosignAnnotationJson, ok := (*fromAnnotations)[cosignAnnotationConversionAnnotation]; ok {
		var cosignAnnotation []CosignAnnotation
		if err := json.Unmarshal([]byte(cosignAnnotationJson), &cosignAnnotation); err != nil {
			return err
		}
		if is.Cosign == nil {
			is.Cosign = &CosignConfig{}
		}
		is.Cosign.Annotations = cosignAnnotation
		delete(ia, cosignAnnotationConversionAnnotation)
	}
	if defaultProcess, ok := (*fromAnnotations)[defaultProcessConversionAnnotation]; ok {
		is.DefaultProcess = defaultProcess
		delete(ia, defaultProcessConversionAnnotation)
	}
	return nil
}

func (is *ImageSpec)convertToAnnotations(toAnnotations map[string]string) error{
	if build := is.Build; build != nil {
		if len(build.Services) > 0 {
			bytes, err := json.Marshal(build.Services)
			if err != nil {
				return err
			}
			toAnnotations[servicesConversionAnnotation] = string(bytes)
		}
		if len(build.Tolerations) > 0 {
			bytes, err := json.Marshal(build.Tolerations)
			if err != nil {
				return err
			}
			toAnnotations[tolerationsConversionAnnotation] = string(bytes)
		}
		if build.NodeSelector != nil {
			if len(build.NodeSelector) > 0 {
				bytes, err := json.Marshal(build.NodeSelector)
				if err != nil {
					return err
				}
				toAnnotations[nodeSelectorConversionAnnotation] = string(bytes)
			}
		}
		if build.Affinity != nil {
			bytes, err := json.Marshal(build.Affinity)
			if err != nil {
				return err
			}
			toAnnotations[affinityConversionAnnotation] = string(bytes)
		}
		if build.RuntimeClassName != nil {
			toAnnotations[runtimeClassNameConversionAnnotation] = *build.RuntimeClassName
		}
		if build.SchedulerName != "" {
			toAnnotations[schedulerNameConversionAnnotation] = build.SchedulerName
		}
		if build.BuildTimeout != nil {
			toAnnotations[buildTimeoutConversionAnnotation] = strconv.FormatInt(*build.BuildTimeout, 10)
		}
	}
	if is.Cache != nil {
		if is.Cache.Volume != nil && is.Cache.Volume.StorageClassName != "" {
			toAnnotations[storageClassNameConversionAnnotation] = is.Cache.Volume.StorageClassName
		}
		if is.Cache.Registry != nil && is.Cache.Registry.Tag != "" {
			toAnnotations[registryTagConversionAnnotation] = is.Cache.Registry.Tag
		}
	}
	if is.ProjectDescriptorPath != "" {
		toAnnotations[projectDescriptorPathConversionAnnotation] = is.ProjectDescriptorPath
	}
	if is.Cosign != nil {
		if len(is.Cosign.Annotations) > 0 {
			bytes, err := json.Marshal(is.Cosign.Annotations)
			if err != nil {
				return err
			}
			toAnnotations[cosignAnnotationConversionAnnotation] = string(bytes)
		}
	}
	if is.DefaultProcess != "" {
		toAnnotations[defaultProcessConversionAnnotation] = is.DefaultProcess
	}
	return nil
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
