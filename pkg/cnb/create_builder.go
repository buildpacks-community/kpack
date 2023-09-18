package cnb

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
	Save(keychain authn.Keychain, tag string, image ggcrv1.Image) (string, error)
}

type LifecycleProvider interface {
	LayerForOS(os string) (ggcrv1.Layer, LifecycleMetadata, error)
}

type RemoteBuilderCreator struct {
	RegistryClient    RegistryClient
	LifecycleProvider LifecycleProvider
	KpackVersion      string
	KeychainFactory   registry.KeychainFactory
}

func (r *RemoteBuilderCreator) CreateBuilder(ctx context.Context, builderKeychain authn.Keychain, stackKeychain authn.Keychain, fetcher RemoteBuildpackFetcher, clusterStack *buildapi.ClusterStack, spec buildapi.BuilderSpec) (buildapi.BuilderRecord, error) {
	buildImage, _, err := r.RegistryClient.Fetch(stackKeychain, clusterStack.Status.BuildImage.LatestImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	builderBldr := newBuilderBldr(r.KpackVersion)

	err = builderBldr.AddStack(buildImage, clusterStack)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	lifecycleLayer, lifecycleMetadata, err := r.LifecycleProvider.LayerForOS(builderBldr.os)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	builderBldr.AddLifecycle(lifecycleLayer, lifecycleMetadata)

	for _, group := range spec.Order {
		buildpacks := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, buildpack := range group.Group {
			remoteBuildpack, err := fetcher.ResolveAndFetch(ctx, buildpack)
			if err != nil {
				return buildapi.BuilderRecord{}, err
			}

			buildpacks = append(buildpacks, remoteBuildpack.Optional(buildpack.Optional))
		}
		builderBldr.AddGroup(buildpacks...)
	}

	for _, group := range spec.OrderExtensions {
		extensions := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, extension := range group.Group {
			remoteExtension, err := fetcher.ResolveAndFetch(ctx, extension)
			if err != nil {
				return buildapi.BuilderRecord{}, err
			}

			extensions = append(extensions, remoteExtension.Optional(true))
		}
		builderBldr.AddExtensionsGroup(extensions...)
	}

	writeableImage, err := builderBldr.WriteableImage()
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	identifier, err := r.RegistryClient.Save(builderKeychain, spec.Tag, writeableImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	config, err := writeableImage.ConfigFile()
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	builder := buildapi.BuilderRecord{
		Image: identifier,
		Stack: corev1alpha1.BuildStack{
			RunImage: clusterStack.Status.RunImage.LatestImage,
			ID:       clusterStack.Status.Id,
		},
		Buildpacks:              buildpackMetadata(builderBldr.buildpacks()),
		Extensions:              buildpackMetadata(builderBldr.extensions()),
		Order:                   builderBldr.order,
		ObservedStackGeneration: clusterStack.Status.ObservedGeneration,
		ObservedStoreGeneration: fetcher.ClusterStoreObservedGeneration(),
		OS:                      config.OS,
	}

	return builder, nil
}

func buildpackMetadata(buildpacks []DescriptiveBuildpackInfo) corev1alpha1.BuildpackMetadataList {
	m := make(corev1alpha1.BuildpackMetadataList, 0, len(buildpacks))
	for _, b := range buildpacks {
		m = append(m, corev1alpha1.BuildpackMetadata{
			Id:       b.Id,
			Version:  b.Version,
			Homepage: b.Homepage,
		})
	}
	return m
}
