package v1alpha1

import (
	"fmt"
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

	BuildReasonConfig    = "CONFIG"
	BuildReasonCommit    = "COMMIT"
	BuildReasonBuildpack = "BUILDPACK"
	BuildReasonStack     = "STACK"
	BuildReasonTrigger   = "TRIGGER"
)

type BuildReason string

func (im *Image) LatestForImage(build *Build) string {
	if build.IsSuccess() {
		return build.BuiltImage()
	}
	return im.Status.LatestImage
}

func (im *Image) Bindings() corev1alpha1.CNBBindings {
	if im.Spec.Build == nil {
		return nil
	}
	return im.Spec.Build.Bindings
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

func (im *Image) CacheName() string {
	return kmeta.ChildName(im.Name, "-cache")
}

func (im *Image) NeedCache() bool {
	return im.Spec.CacheSize != nil
}

func (im *Image) BuildCache() *corev1.PersistentVolumeClaim {
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
					corev1.ResourceStorage: *im.Spec.CacheSize,
				},
			},
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
			ServiceAccount: im.Spec.ServiceAccount,
			Source:         im.Spec.Source,
		},
	}
}

func (im *Image) generateTags(buildNumber string) []string {
	if im.disableAdditionalImageNames() {
		return []string{im.Spec.Tag}
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
	return []string{
		im.Spec.Tag,
		tag.RegistryStr() + "/" + tag.RepositoryStr() + ":" + tagName + "b" + buildNumber + "." + now.Format("20060102") + "." + fmt.Sprintf("%02d%02d%02d", now.Hour(), now.Minute(), now.Second())}
}

func (im *Image) generateBuildName(buildNumber string) string {
	return im.Name + "-build-" + buildNumber + "-"
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
