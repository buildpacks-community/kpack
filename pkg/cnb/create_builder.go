package cnb

import (
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	experimentalV1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

type Client interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, error)
	Save(keychain authn.Keychain, tag string, image v1.Image) (string, error)
}

type StoreFactory interface {
	MakeStore(keychain authn.Keychain, storeImage string) (Store, error)
}

type Store interface {
	FetchBuildpack(id, version string) (RemoteBuildpackInfo, error)
}

type RemoteBuilderCreator struct {
	RemoteImageClient Client
	StoreFactory      StoreFactory
}

func (r *RemoteBuilderCreator) CreateBuilder(keychain authn.Keychain, customBuilder *experimentalV1alpha1.CustomBuilder) (v1alpha1.BuilderRecord, error) {
	baseImage, err := r.RemoteImageClient.Fetch(keychain, customBuilder.Spec.Stack.BaseBuilderImage)
	if err != nil {
		return emptyRecord, err
	}

	store, err := r.StoreFactory.MakeStore(keychain, customBuilder.Spec.Store.Image)
	if err != nil {
		return emptyRecord, err
	}

	builderBuilder, err := newBuilderBuilder(baseImage)
	if err != nil {
		return emptyRecord, err
	}

	for _, group := range customBuilder.Spec.Order {
		buildpacks := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, buildpack := range group.Group {
			remoteBuildpack, err := store.FetchBuildpack(buildpack.ID, buildpack.Version)
			if err != nil {
				return emptyRecord, err
			}

			buildpacks = append(buildpacks, remoteBuildpack.Optional(buildpack.Optional))
		}
		builderBuilder.addGroup(buildpacks...)
	}

	writeableImage, err := builderBuilder.writeableImage()
	if err != nil {
		return emptyRecord, err
	}

	identifier, err := r.RemoteImageClient.Save(keychain, customBuilder.Spec.Tag, writeableImage)
	if err != nil {
		return emptyRecord, err
	}

	return v1alpha1.BuilderRecord{
		Image:      identifier,
		Stack:      builderBuilder.stack(),
		Buildpacks: builderBuilder.buildpacks(),
	}, nil
}
