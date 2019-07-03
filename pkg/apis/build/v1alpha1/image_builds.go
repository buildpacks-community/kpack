package v1alpha1

import (
	"strconv"

	"github.com/knative/pkg/kmeta"
	"github.com/pborman/uuid"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return &Build{
		ObjectMeta: v1.ObjectMeta{
			Name: im.generateBuildName(),
			OwnerReferences: []v1.OwnerReference{
				*kmeta.NewControllerRef(im),
			},
			Labels: map[string]string{
				BuildNumberLabel: im.nextBuildNumber(),
				ImageLabel:       im.Name,
			},
		},
		Spec: BuildSpec{
			Image:          im.Spec.Image,
			Builder:        builder.Spec.Image,
			ServiceAccount: im.Spec.ServiceAccount,
			Source: Source{
				Git: Git{
					URL:      sourceResolver.Status.ResolvedSource.Git.URL,
					Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
				},
			},
			CacheName: im.Status.BuildCacheName,
		},
	}
}

func (im *Image) NeedCache() bool {
	return im.Spec.CacheSize != nil
}

func (im *Image) MakeBuildCache() *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      im.CacheName(),
			Namespace: im.Namespace,
			OwnerReferences: []v1.OwnerReference{
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

func (im *Image) generateBuildName() string {
	name := im.Name + "-build-" + im.nextBuildNumber() + "-" + uuid.New()
	if len(name) > 64 {
		return name[:63]
	}

	return name
}
