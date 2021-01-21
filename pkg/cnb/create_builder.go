package cnb

import (
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
	Save(keychain authn.Keychain, tag string, image v1.Image) (string, error)
}

type BuildpackRepository interface {
	FindByIdAndVersion(id, version string) (RemoteBuildpackInfo, error)
}

type NewBuildpackRepository func(clusterStore *v1alpha1.ClusterStore) BuildpackRepository

type RemoteBuilderCreator struct {
	RegistryClient         RegistryClient
	LifecycleImage         string
	NewBuildpackRepository NewBuildpackRepository
	KpackVersion           string
}

func (r *RemoteBuilderCreator) CreateBuilder(keychain authn.Keychain, clusterStore *v1alpha1.ClusterStore, clusterStack *v1alpha1.ClusterStack, spec v1alpha1.BuilderSpec) (v1alpha1.BuilderRecord, error) {
	buildpackRepo := r.NewBuildpackRepository(clusterStore)

	buildImage, _, err := r.RegistryClient.Fetch(keychain, clusterStack.Status.BuildImage.LatestImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	lifecycleImage, _, err := r.RegistryClient.Fetch(keychain, r.LifecycleImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	builderBldr, err := newBuilderBldr(lifecycleImage, r.KpackVersion)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	err = builderBldr.AddStack(buildImage, clusterStack)
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
		builderBldr.AddGroup(buildpacks...)
	}

	writeableImage, err := builderBldr.WriteableImage()
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	identifier, err := r.RegistryClient.Save(keychain, spec.Tag, writeableImage)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	config, err := writeableImage.ConfigFile()
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	return v1alpha1.BuilderRecord{
		Image: identifier,
		Stack: v1alpha1.BuildStack{
			RunImage: clusterStack.Status.RunImage.LatestImage,
			ID:       clusterStack.Status.Id,
		},
		Buildpacks:              buildpackMetadata(builderBldr.buildpacks()),
		Order:                   builderBldr.order,
		ObservedStackGeneration: clusterStack.Status.ObservedGeneration,
		ObservedStoreGeneration: clusterStore.Status.ObservedGeneration,
		OS:                      config.OS,
	}, nil
}

func buildpackMetadata(buildpacks []DescriptiveBuildpackInfo) v1alpha1.BuildpackMetadataList {
	m := make(v1alpha1.BuildpackMetadataList, 0, len(buildpacks))
	for _, b := range buildpacks {
		m = append(m, v1alpha1.BuildpackMetadata{
			Id:       b.Id,
			Version:  b.Version,
			Homepage: b.Homepage,
		})
	}
	return m
}
