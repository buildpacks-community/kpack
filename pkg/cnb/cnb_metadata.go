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
	BuildpackMetadata []lifecycle.GroupBuildpack
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
