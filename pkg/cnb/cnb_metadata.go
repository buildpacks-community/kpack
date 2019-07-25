package cnb

import (
	"encoding/json"
	"time"

	lcyclemd "github.com/buildpack/lifecycle/metadata"

	"github.com/pivotal/build-service-beam/pkg/registry"
)

const BuilderMetadataLabel = "io.buildpacks.builder.metadata"

type BuilderBuildpackMetadata struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type BuilderImageMetadata struct {
	Buildpacks []BuilderBuildpackMetadata `json:"buildpacks"`
}

type BuilderMetadata []BuilderBuildpackMetadata

type RemoteMetadataRetriever struct {
	LifecycleImageFactory registry.RemoteImageFactory
}

func (r *RemoteMetadataRetriever) GetBuilderBuildpacks(repo registry.ImageRef) (BuilderMetadata, error) {
	img, err := r.LifecycleImageFactory.NewRemote(repo)
	if err != nil {
		return nil, err
	}

	var metadataJSON string
	metadataJSON, err = img.Label(BuilderMetadataLabel)
	if err != nil {
		return nil, err
	}

	var metadata BuilderImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return nil, err
	}

	return metadata.Buildpacks, nil
}

func (r *RemoteMetadataRetriever) GetBuiltImage(ref registry.ImageRef) (BuiltImage, error) {
	img, err := r.LifecycleImageFactory.NewRemote(ref)
	if err != nil {
		return BuiltImage{}, err
	}

	var metadataJSON string
	metadataJSON, err = img.Label(lcyclemd.AppMetadataLabel)
	if err != nil {
		return BuiltImage{}, err
	}

	var metadata lcyclemd.AppImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return BuiltImage{}, err
	}

	imageCreatedAt, err := img.CreatedAt()
	if err != nil {
		return BuiltImage{}, err
	}

	digest, err := img.Digest()
	if err != nil {
		return BuiltImage{}, err
	}

	return BuiltImage{
		SHA:               digest,
		CompletedAt:       imageCreatedAt,
		BuildpackMetadata: metadata.Buildpacks,
	}, nil
}

type BuiltImage struct {
	SHA               string
	CompletedAt       time.Time
	BuildpackMetadata []lcyclemd.BuildpackMetadata
}
