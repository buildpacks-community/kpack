package cnb

import (
	"time"

	"github.com/buildpacks/lifecycle"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	BuilderMetadataLabel = "io.buildpacks.builder.metadata"
)

type FetchableBuilder interface {
	metav1.ObjectMetaAccessor
	Image() string
	ImagePullSecrets() []v1.LocalObjectReference
}

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
}

type RemoteMetadataRetriever struct {
	KeychainFactory registry.KeychainFactory
	ImageFetcher    ImageFetcher
}

func (r *RemoteMetadataRetriever) GetBuilderImage(builder FetchableBuilder) (v1alpha1.BuilderRecord, error) {
	builderKeychain, err := r.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		Namespace:        builder.GetObjectMeta().GetNamespace(),
		ImagePullSecrets: builder.ImagePullSecrets(),
	})
	if err != nil {
		return v1alpha1.BuilderRecord{}, errors.Wrap(err, "unable to create builder image keychain")
	}

	builderImage, builderImageId, err := r.ImageFetcher.Fetch(builderKeychain, builder.Image())
	if err != nil {
		return v1alpha1.BuilderRecord{}, errors.Wrap(err, "unable to fetch remote builder image")
	}

	stackId, err := imagehelpers.GetStringLabel(builderImage, lifecycle.StackIDLabel)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	var md BuilderImageMetadata
	err = imagehelpers.GetLabel(builderImage, BuilderMetadataLabel, &md)
	if err != nil {
		return v1alpha1.BuilderRecord{}, errors.Wrap(err, "unsupported builder metadata structure")
	}

	runImageKeychain, err := r.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		Namespace:        builder.GetObjectMeta().GetNamespace(),
		ImagePullSecrets: builder.ImagePullSecrets(),
	})
	if err != nil {
		return v1alpha1.BuilderRecord{}, errors.Wrap(err, "unable to create run image keychain")
	}

	_, runImageId, err := r.ImageFetcher.Fetch(runImageKeychain, md.Stack.RunImage.Image)
	if err != nil {
		return v1alpha1.BuilderRecord{}, errors.Wrap(err, "unable to fetch remote run image")
	}

	return v1alpha1.BuilderRecord{
		Image: builderImageId,
		Stack: v1alpha1.BuildStack{
			RunImage: runImageId,
			ID:       stackId,
		},
		Buildpacks: transform(md.Buildpacks),
	}, nil
}

func transform(infos []expv1alpha1.BuildpackInfo) v1alpha1.BuildpackMetadataList {
	buildpacks := make(v1alpha1.BuildpackMetadataList, 0, len(infos))
	for _, buildpack := range infos {
		buildpacks = append(buildpacks, v1alpha1.BuildpackMetadata{
			Id:      buildpack.Id,
			Version: buildpack.Version,
		})
	}
	return buildpacks
}

func (r *RemoteMetadataRetriever) GetBuiltImage(build *v1alpha1.Build) (BuiltImage, error) {
	keychain, err := r.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		ServiceAccount: build.Spec.ServiceAccount,
		Namespace:      build.Namespace,
	})
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "unable to create app image keychain")
	}

	appImage, appImageId, err := r.ImageFetcher.Fetch(keychain, build.Tag())
	if err != nil {
		return BuiltImage{}, errors.Wrap(err, "unable to fetch app image")
	}

	return readBuiltImage(appImage, appImageId)
}

type BuiltImage struct {
	Identifier        string
	CompletedAt       time.Time
	BuildpackMetadata []lifecycle.Buildpack
	Stack             BuiltImageStack
}

func readBuiltImage(appImage ggcrv1.Image, appImageId string) (BuiltImage, error) {
	stackId, err := imagehelpers.GetStringLabel(appImage, lifecycle.StackIDLabel)
	if err != nil {
		return BuiltImage{}, nil
	}

	var buildMetadata lifecycle.BuildMetadata
	err = imagehelpers.GetLabel(appImage, lifecycle.BuildMetadataLabel, &buildMetadata)
	if err != nil {
		return BuiltImage{}, err
	}

	var layerMetadata appLayersMetadata
	err = imagehelpers.GetLabel(appImage, lifecycle.LayerMetadataLabel, &layerMetadata)
	if err != nil {
		return BuiltImage{}, err
	}

	imageCreatedAt, err := imagehelpers.GetCreatedAt(appImage)
	if err != nil {
		return BuiltImage{}, err
	}

	runImageRef, err := name.ParseReference(layerMetadata.RunImage.Reference)
	if err != nil {
		return BuiltImage{}, err
	}

	baseImageRef, err := name.ParseReference(layerMetadata.Stack.RunImage.Image)
	if err != nil {
		return BuiltImage{}, err
	}

	return BuiltImage{
		Identifier:        appImageId,
		CompletedAt:       imageCreatedAt,
		BuildpackMetadata: buildMetadata.Buildpacks,
		Stack: BuiltImageStack{
			RunImage: baseImageRef.Context().String() + "@" + runImageRef.Identifier(),
			ID:       stackId,
		},
	}, nil
}

type appLayersMetadata struct {
	RunImage runImageAppMetadata `json:"runImage" toml:"run-image"`
	Stack    StackMetadata       `json:"stack" toml:"stack"`
}

type runImageAppMetadata struct {
	TopLayer  string `json:"topLayer" toml:"top-layer"`
	Reference string `json:"reference" toml:"reference"`
}
