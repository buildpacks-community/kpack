package v1alpha2

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	BuildNumberLabel     = "image.kpack.io/buildNumber"
	ImageLabel           = "image.kpack.io/image"
	ImageGenerationLabel = "image.kpack.io/imageGeneration"

	BuildReasonAnnotation  = "image.kpack.io/reason"
	BuildChangesAnnotation = "image.kpack.io/buildChanges"
	BuildNeededAnnotation  = "image.kpack.io/additionalBuildNeeded"

	BuilderNameAnnotation = "image.kpack.io/builderName"
	BuilderKindAnnotation = "image.kpack.io/builderKind"

	BuildReasonConfig    = "CONFIG"
	BuildReasonCommit    = "COMMIT"
	BuildReasonBuildpack = "BUILDPACK"
	BuildReasonStack     = "STACK"
	BuildReasonTrigger   = "TRIGGER"
)

type BuildReason string

func (im *Image) Build(sourceResolver *SourceResolver, builder BuilderResource, latestBuild *Build, reasons, changes string, nextBuildNumber int64, priorityClass string) *Build {
	buildNumber := strconv.Itoa(int(nextBuildNumber))
	return &Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: im.Namespace,
			Name:      im.generateBuildName(buildNumber),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(im),
			},
			Labels: combine(im.Labels, map[string]string{
				BuildNumberLabel:     buildNumber,
				ImageLabel:           im.Name,
				ImageGenerationLabel: strconv.Itoa(int(im.Generation)),
			}),
			Annotations: combine(im.Annotations, map[string]string{
				BuildReasonAnnotation:  reasons,
				BuildChangesAnnotation: changes,
				BuilderNameAnnotation:  builder.GetName(),
				BuilderKindAnnotation:  builder.GetKind(),
			}),
		},
		Spec: BuildSpec{
			Tags:    im.generateTags(buildNumber),
			Builder: builder.BuildBuilderSpec(),
			RunImage: BuildSpecImage{
				Image: builder.RunImage(),
			},
			ServiceAccountName:    im.Spec.ServiceAccountName,
			Source:                sourceResolver.SourceConfig(),
			Cache:                 im.getBuildCacheConfig(),
			Services:              im.Services(),
			CNBBindings:           im.CNBBindings(),
			Env:                   im.Env(),
			ProjectDescriptorPath: im.Spec.ProjectDescriptorPath,
			Resources:             im.Resources(),
			LastBuild:             lastBuild(latestBuild),
			Notary:                im.Spec.Notary,
			Cosign:                im.Spec.Cosign,
			DefaultProcess:        im.Spec.DefaultProcess,
			Tolerations:           im.Tolerations(),
			NodeSelector:          im.NodeSelector(),
			Affinity:              im.Affinity(),
			RuntimeClassName:      im.RuntimeClassName(),
			SchedulerName:         im.SchedulerName(),
			PriorityClassName:     priorityClass,
			ActiveDeadlineSeconds: im.BuildTimeout(),
			CreationTime:          im.Spec.Build.CreationTime,
		},
	}
}

func (is *ImageSpec) NeedVolumeCache() bool {
	return is.Cache != nil && is.Cache.Volume != nil && is.Cache.Volume.Size != nil
}

func (is *ImageSpec) NeedRegistryCache() bool {
	return is.Cache != nil && is.Cache.Registry != nil && is.Cache.Registry.Tag != ""
}

func (im *Image) getBuildCacheConfig() *BuildCacheConfig {
	buildCacheConfig := BuildCacheConfig{}

	if im.Spec.NeedRegistryCache() {
		buildCacheConfig.Registry = im.Spec.Cache.Registry.DeepCopy()
	}

	if im.Spec.NeedVolumeCache() {
		buildCacheConfig.Volume = &BuildPersistentVolumeCache{ClaimName: im.Status.BuildCacheName}
	}

	return &buildCacheConfig
}

func lastBuild(latestBuild *Build) *LastBuild {
	if latestBuild == nil {
		return nil
	}

	if latestBuild.IsFailure() {
		return latestBuild.Spec.LastBuild
	}

	return &LastBuild{
		Image:   latestBuild.BuiltImage(),
		Cache:   BuildCache{Image: latestBuild.CacheImage()},
		StackId: latestBuild.Stack(),
	}
}

func (im *Image) LatestForImage(build *Build) string {
	if build.IsSuccess() {
		return build.BuiltImage()
	}
	return im.Status.LatestImage
}

func (im *Image) Services() Services {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.Services
}

func (im *Image) CNBBindings() corev1alpha1.CNBBindings {
	if im.Spec.Build == nil {
		return nil
	}

	return im.Spec.Build.CNBBindings
}

func (im *Image) Env() []corev1.EnvVar {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.Env
}

func (im *Image) Resources() corev1.ResourceRequirements {
	if im.Spec.Build == nil {
		return corev1.ResourceRequirements{}
	}
	return im.Spec.Build.Resources
}

func (im *Image) Tolerations() []corev1.Toleration {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.Tolerations
}

func (im *Image) NodeSelector() map[string]string {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.NodeSelector
}

func (im *Image) Affinity() *corev1.Affinity {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.Affinity
}

func (im *Image) BuildTimeout() *int64 {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.BuildTimeout
}

func (im *Image) RuntimeClassName() *string {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.RuntimeClassName
}

func (im *Image) SchedulerName() string {
	if im.Spec.Build == nil {
		return ""
	}
	return im.Spec.Build.SchedulerName
}

func (im *Image) CacheName() string {
	return kmeta.ChildName(im.Name, "-cache")
}

func (im *Image) BuildCache() *corev1.PersistentVolumeClaim {
	var storageClassName *string
	if im.Spec.Cache.Volume.StorageClassName != "" {
		storageClassName = &im.Spec.Cache.Volume.StorageClassName
	}
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      im.CacheName(),
			Namespace: im.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(im),
			},
			Labels: im.Labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *im.Spec.Cache.Volume.Size,
				},
			},
			StorageClassName: storageClassName,
		},
	}
}

func (im *Image) SourceResolverName() string {
	return kmeta.ChildName(im.Name, "-source")
}

func (im *Image) SourceResolver() *SourceResolver {
	return &SourceResolver{
		ObjectMeta: metav1.ObjectMeta{
			Name:      im.SourceResolverName(),
			Namespace: im.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(im),
			},
			Labels: im.Labels,
		},
		Spec: SourceResolverSpec{
			ServiceAccountName: im.Spec.ServiceAccountName,
			Source:             im.Spec.Source,
		},
	}
}

func (im *Image) generateTags(buildNumber string) []string {
	if im.disableAdditionalImageNames() {
		return append([]string{im.Spec.Tag}, im.Spec.AdditionalTags...)
	}
	now := time.Now()

	tag, err := name.NewTag(im.Spec.Tag, name.WeakValidation)
	if err != nil {
		// We assume that if the Image Name cannot be parsed the image will not be successfully built
		// in this case we can just ignore any additional image names
		return nil
	}

	tagName := tag.TagStr() + "-"
	if tagName == "latest-" {
		tagName = ""
	}
	return append([]string{
		im.Spec.Tag,
		tag.RegistryStr() + "/" + tag.RepositoryStr() + ":" + tagName + "b" + buildNumber + "." + now.Format("20060102") + "." + fmt.Sprintf("%02d%02d%02d", now.Hour(), now.Minute(), now.Second())},
		im.Spec.AdditionalTags...,
	)
}

func (im *Image) generateBuildName(buildNumber string) string {
	return kmeta.ChildName(im.Name, "-build-"+buildNumber)
}

func combine(map1, map2 map[string]string) map[string]string {
	combinedMap := make(map[string]string, len(map1)+len(map2))

	for k, v := range map1 {
		combinedMap[k] = v
	}
	for k, v := range map2 {
		combinedMap[k] = v
	}
	return combinedMap
}

func (im *Image) disableAdditionalImageNames() bool {
	return im.Spec.ImageTaggingStrategy == corev1alpha1.None
}
