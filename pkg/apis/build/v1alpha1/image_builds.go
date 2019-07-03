package v1alpha1

import (
	"fmt"
	"strconv"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/knative/pkg/kmeta"
	"github.com/pborman/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	BuildNumberLabel = "image.build.pivotal.io/buildNumber"
	ImageLabel       = "image.build.pivotal.io/image"
)

func (im *Image) BuildNeeded(sourceResolver *SourceResolver, lastBuild *Build, builder *Builder) bool {
	if !sourceResolver.Ready() {
		return false
	}

	if lastBuild == nil {
		return true
	}

	if im.lastBuildMatchesDesiredBuild(sourceResolver, lastBuild) && lastBuildBuiltWithBuilderBuildpacks(builder, lastBuild) {
		return false
	}

	return true
}

func (im *Image) lastBuildMatchesDesiredBuild(sourceResolver *SourceResolver, build *Build) bool {
	if sourceResolver.Status.ResolvedSource.Git.URL != build.Spec.Source.Git.URL {
		return false
	}

	if sourceResolver.Status.ResolvedSource.Git.Revision != build.Spec.Source.Git.Revision {
		return false
	}

	return im.Spec.Image == build.Spec.Image
}

func lastBuildBuiltWithBuilderBuildpacks(builder *Builder, build *Build) bool {
	for _, bp := range build.Status.BuildMetadata {
		if !builder.Status.BuilderMetadata.Include(bp) {
			return false
		}
	}

	return true
}

func (im *Image) CreateBuild(sourceResolver *SourceResolver, builder *Builder) *Build {
	nextBuildNumber := im.nextBuildNumber()
	return &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: im.generateBuildName(),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(im),
			},
			Labels: map[string]string{
				BuildNumberLabel: nextBuildNumber,
				ImageLabel:       im.Name,
			},
		},
		Spec: BuildSpec{
			Image:                im.Spec.Image,
			Builder:              builder.Spec.Image,
			ServiceAccount:       im.Spec.ServiceAccount,
			Source:               Source{
				Git: Git{
					URL:      sourceResolver.Status.ResolvedSource.Git.URL,
					Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
				},
			},
			CacheName:            im.Status.BuildCacheName,
			AdditionalImageNames: im.generateImageNames(nextBuildNumber),
		},
	}
}

func (im *Image) NeedCache() bool {
	return im.Spec.CacheSize != nil
}

func (im *Image) MakeBuildCache() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      im.CacheName(),
			Namespace: im.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(im),
			},
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

func (im *Image) nextBuildNumber() string {
	return strconv.Itoa(int(im.Status.BuildCounter + 1))
}

func (im *Image) generateImageNames(buildNumber string) []string {
	if im.Spec.DisableAdditionalImageNames {
		return nil
	}
	now := time.Now()

	tag, err := name.NewTag(im.Spec.Image, name.WeakValidation)
	if err != nil {
		// We assume that if the Image Name cannot be parsed the image will not be successfully built
		// in this case we can just ignore any additional image names
		return nil
	}

	tagName := tag.TagStr() + "-"
	if tagName == "latest-" {
		tagName = ""
	}
	return []string{tag.RegistryStr() + "/" + tag.RepositoryStr() + ":" + tagName + "b" + buildNumber + "." + now.Format("20060102") + "." + fmt.Sprintf("%02d%02d%02d", now.Hour(), now.Minute(), now.Second())}
}

func (im *Image) generateBuildName() string {
	name := im.Name + "-build-" + im.nextBuildNumber() + "-" + uuid.New()
	if len(name) > 64 {
		return name[:63]
	}

	return name
}
