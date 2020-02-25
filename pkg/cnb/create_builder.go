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

type BuildpackRepository interface {
	FindByIdAndVersion(id, version string) (RemoteBuildpackInfo, error)
}

type RemoteBuilderCreator struct {
	RegistryClient RegistryClient
	LifecycleImage string
	KpackVersion   string
}

func (r *RemoteBuilderCreator) CreateBuilder(keychain authn.Keychain, buildpackRepo BuildpackRepository, stack *expv1alpha1.Stack, spec expv1alpha1.CustomBuilderSpec) (v1alpha1.BuilderRecord, error) {
	buildImage, _, err := r.RegistryClient.Fetch(keychain, stack.Status.BuildImage.LatestImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	lifecycleImage, _, err := r.RegistryClient.Fetch(keychain, r.LifecycleImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	builderBuilder, err := newBuilderBuilder(buildImage, lifecycleImage, stack, r.KpackVersion)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	for _, group := range spec.Order {
		buildpacks := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, buildpack := range group.Group {
			remoteBuildpack, err := buildpackRepo.FindByIdAndVersion(buildpack.Id, buildpack.Version)
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
			RunImage: stack.Status.RunImage.LatestImage,
			ID:       stack.Spec.Id,
		},
		Buildpacks: buildpackMetadata(builderBuilder.buildpacks()),
	}, nil
}

func buildpackMetadata(buildpacks []expv1alpha1.BuildpackInfo) v1alpha1.BuildpackMetadataList {
	m := make(v1alpha1.BuildpackMetadataList, 0, len(buildpacks))
	for _, b := range buildpacks {
		m = append(m, v1alpha1.BuildpackMetadata{
			Id:      b.Id,
			Version: b.Version,
		})
	}
	return m
}
