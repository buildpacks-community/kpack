package cnb

import (
	"encoding/json"
	"time"

	lcyclemd "github.com/buildpack/lifecycle/metadata"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
)

const BuilderMetadataLabel = "io.buildpacks.builder.metadata"

type BuildpackMetadata struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type BuilderImageMetadata struct {
	Buildpacks []BuildpackMetadata `json:"buildpacks"`
}

type BuilderImage struct {
	BuilderBuildpackMetadata BuilderMetadata
	Identifier               string
}

type BuilderMetadata []BuildpackMetadata

type RemoteMetadataRetriever struct {
	RemoteImageFactory registry.RemoteImageFactory
}

func (r *RemoteMetadataRetriever) GetBuilderImage(builder v1alpha1.BuilderResource) (BuilderImage, error) {
	img, err := r.RemoteImageFactory.NewRemote(builder.Image(), registry.SecretRef{
		Namespace:        builder.GetObjectMeta().GetNamespace(),
		ImagePullSecrets: builder.ImagePullSecrets(),
	})
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "unable to fetch remote builder image")
	}

	var metadataJSON string
	metadataJSON, err = img.Label(BuilderMetadataLabel)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "builder image metadata label not present")
	}

	var metadata BuilderImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "unsupported builder metadata structure")
	}

	identifier, err := img.Identifier()
	if err != nil {
		return BuilderImage{}, errors.Wrap(err, "failed to retrieve builder image SHA")
	}

	return BuilderImage{
		BuilderBuildpackMetadata: metadata.Buildpacks,
		Identifier:               identifier,
	}, nil
}

func (r *RemoteMetadataRetriever) GetBuiltImage(ref *v1alpha1.Build) (BuiltImage, error) {
	img, err := r.RemoteImageFactory.NewRemote(ref.Tag(), registry.SecretRef{
		ServiceAccount: ref.Spec.ServiceAccount,
		Namespace:      ref.Namespace,
	})
	if err != nil {
		return BuiltImage{}, err
	}

	var metadataJSON string
	metadataJSON, err = img.Label(lcyclemd.BuildMetadataLabel)
	if err != nil {
		return BuiltImage{}, err
	}

	var metadata lcyclemd.BuildMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuiltImage{}, err
	}

	imageCreatedAt, err := img.CreatedAt()
	if err != nil {
		return BuiltImage{}, err
	}

	identifier, err := img.Identifier()
	if err != nil {
		return BuiltImage{}, err
	}

	return BuiltImage{
		Identifier:        identifier,
		CompletedAt:       imageCreatedAt,
		BuildpackMetadata: metadata.Buildpacks,
	}, nil
}

type BuiltImage struct {
	Identifier        string
	CompletedAt       time.Time
	BuildpackMetadata []lcyclemd.BuildpackMetadata
}
