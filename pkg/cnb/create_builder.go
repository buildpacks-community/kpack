package cnb

import (
	"context"
	"errors"

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

	// fetch and add buildpacks
	for _, group := range spec.Order {
		buildpacks := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, bp := range group.Group {
			remoteBuildpack, err := fetcher.ResolveAndFetchBuildpack(ctx, bp)
			if err != nil {
				return buildapi.BuilderRecord{}, err
			}

			buildpacks = append(buildpacks, remoteBuildpack.Optional(bp.Optional))
		}
		builderBldr.AddBuildpackGroup(buildpacks...)
	}

	// fetch and add extensions
	if builderBldr.os == "windows" && len(spec.OrderExtensions) > 0 {
		return buildapi.BuilderRecord{}, errors.New("image extensions are not supported for Windows builds")
	}
	for _, group := range spec.OrderExtensions {
		extensions := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, ext := range group.Group {
			remoteExtension, err := fetcher.ResolveAndFetchExtension(ctx, ext)
			if err != nil {
				return buildapi.BuilderRecord{}, err
			}

			extensions = append(extensions, remoteExtension.Optional(true)) // extensions are always optional
		}
		builderBldr.AddExtensionGroup(extensions...)
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
		Order:                   builderBldr.order,
		OrderExtensions:         builderBldr.orderExtensions,
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
