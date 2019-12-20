package cnb

import (
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
	Save(keychain authn.Keychain, tag string, image v1.Image) (string, error)
}

type Store interface {
	FetchBuildpack(id, version string) (RemoteBuildpackInfo, error)
}

type RemoteBuilderCreator struct {
	RegistryClient RegistryClient
}

func (r *RemoteBuilderCreator) CreateBuilder(keychain authn.Keychain, store Store, spec expv1alpha1.CustomBuilderSpec) (v1alpha1.BuilderRecord, error) {
	baseImage, _, err := r.RegistryClient.Fetch(keychain, spec.Stack.BaseBuilderImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	builderBuilder, err := newBuilderBuilder(baseImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	_, runImageId, err := r.RegistryClient.Fetch(keychain, builderBuilder.baseMetadata.Stack.RunImage.Image)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	for _, group := range spec.Order {
		buildpacks := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, buildpack := range group.Group {
			remoteBuildpack, err := store.FetchBuildpack(buildpack.ID, buildpack.Version)
			if err != nil {
				return v1alpha1.BuilderRecord{}, err
			}

			buildpacks = append(buildpacks, remoteBuildpack.Optional(buildpack.Optional))
		}
		builderBuilder.addGroup(buildpacks...)
	}

	writeableImage, err := builderBuilder.writeableImage()
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	identifier, err := r.RegistryClient.Save(keychain, spec.Tag, writeableImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	return v1alpha1.BuilderRecord{
		Image: identifier,
		Stack: v1alpha1.BuildStack{
			RunImage: runImageId,
			ID:       builderBuilder.stackID,
		},
		Buildpacks: builderBuilder.buildpacks(),
	}, nil
}
