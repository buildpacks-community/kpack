package registry

import (
	"encoding/json"
	"time"

	lcyclemd "github.com/buildpack/lifecycle/metadata"
	"github.com/google/go-containerregistry/pkg/authn"
)

const BuilderMetadataLabel = "io.buildpacks.builder.metadata"

type RemoteImage interface {
	CreatedAt() (time.Time, error)
	Digest() (string, error)
	Label(labelName string) (string, error)
}

//go:generate counterfeiter . Factory
type Factory interface {
	NewRemote(imageRef ImageRef) (RemoteImage, error)
}

type BuilderBuildpackMetadata struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type BuilderImageMetadata struct {
	Buildpacks []BuilderBuildpackMetadata `json:"buildpacks"`
}

type BuilderMetadata []BuilderBuildpackMetadata

type RemoteMetadataRetriever struct {
	LifecycleImageFactory Factory
}

func (r *RemoteMetadataRetriever) GetBuilderBuildpacks(repo ImageRef) (BuilderMetadata, error) {
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

func (r *RemoteMetadataRetriever) GetBuiltImage(ref ImageRef) (BuiltImage, error) {
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

type ImageRef interface {
	ServiceAccount() string
	Namespace() string
	RepoName() string
}

type noAuthImageRef struct {
	repoName string
}

func NewNoAuthImageRef(repoName string) *noAuthImageRef {
	return &noAuthImageRef{repoName: repoName}
}

func (na *noAuthImageRef) RepoName() string {
	return na.repoName
}

func (noAuthImageRef) ServiceAccount() string {
	return ""
}

func (noAuthImageRef) Namespace() string {
	return ""
}

type KeychainFactory interface {
	KeychainForImageRef(ImageRef) authn.Keychain
}
