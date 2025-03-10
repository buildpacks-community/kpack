package cnb

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	corev1 "k8s.io/api/core/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/cosign"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
	Save(keychain authn.Keychain, tag string, image ggcrv1.Image) (string, error)
}

type LifecycleProvider interface {
	Layer() (ggcrv1.Layer, LifecycleMetadata, error)
}

type RemoteBuilderCreator struct {
	RegistryClient RegistryClient
	KpackVersion   string
	ImageSigner    cosign.BuilderSigner
}

func (r *RemoteBuilderCreator) CreateBuilder(
	ctx context.Context,
	builderKeychain authn.Keychain,
	stackKeychain authn.Keychain,
	lifecycleKeychain authn.Keychain,
	fetcher RemoteBuildpackFetcher,
	clusterStack *buildapi.ClusterStack,
	clusterLifecycle *buildapi.ClusterLifecycle,
	spec buildapi.BuilderSpec,
	serviceAccountSecrets []*corev1.Secret,
	resolvedBuilderRef string,
) (buildapi.BuilderRecord, error) {
	buildImage, _, err := r.RegistryClient.Fetch(stackKeychain, clusterStack.Status.BuildImage.LatestImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}
	runImage, _, err := r.RegistryClient.Fetch(stackKeychain, clusterStack.Status.RunImage.LatestImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}
	lifecycleImage, _, err := r.RegistryClient.Fetch(lifecycleKeychain, clusterLifecycle.Spec.Image) // TODO: confirm, should there be a "latest image" on the status?
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	builderBldr := newBuilderBldr(r.KpackVersion)

	relocatedRunImage, err := r.RegistryClient.Save(builderKeychain, fmt.Sprintf("%s-run-image", resolvedBuilderRef), runImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}
	builderBldr.AddRunImage(relocatedRunImage)

	err = builderBldr.AddStack(buildImage, clusterStack)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	lifecycleLayer, lifecycleMetadata, err := layerForOS(clusterLifecycle, lifecycleImage, builderBldr)
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

	builderBldr.AddAdditionalLabels(spec.AdditionalLabels)

	writeableImage, err := builderBldr.WriteableImage()
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	identifier, err := r.RegistryClient.Save(builderKeychain, resolvedBuilderRef, writeableImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	config, err := writeableImage.ConfigFile()
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	var (
		signaturePaths = make([]buildapi.CosignSignature, 0)
	)

	if len(serviceAccountSecrets) > 0 {
		signaturePaths, err = r.ImageSigner.SignBuilder(ctx, identifier, serviceAccountSecrets, builderKeychain)
		if err != nil {
			return buildapi.BuilderRecord{}, err
		}
	}

	builder := buildapi.BuilderRecord{
		Image: identifier,
		Stack: corev1alpha1.BuildStack{
			RunImage: relocatedRunImage,
			ID:       clusterStack.Status.Id,
		},
		Lifecycle: buildapi.ResolvedClusterLifecycle{
			Version: clusterLifecycle.Status.ResolvedClusterLifecycle.Version,
			API:     clusterLifecycle.Status.ResolvedClusterLifecycle.API,
			APIs:    clusterLifecycle.Status.ResolvedClusterLifecycle.APIs,
		},
		Buildpacks:              buildpackMetadata(builderBldr.buildpacks()),
		Order:                   builderBldr.order,
		ObservedStackGeneration: clusterStack.Status.ObservedGeneration,
		ObservedStoreGeneration: fetcher.ClusterStoreObservedGeneration(),
		OS:                      config.OS,
		SignaturePaths:          signaturePaths,
	}

	return builder, nil
}

func layerForOS(clusterLifecycle *buildapi.ClusterLifecycle, lifecycleImage ggcrv1.Image, builderBlder *builderBlder) (lifecycleLayer ggcrv1.Layer, lifecycleMetadata LifecycleMetadata, err error) {
	lifecycleMetadata = LifecycleMetadata{
		LifecycleInfo: LifecycleInfo{
			Version: clusterLifecycle.Status.ResolvedClusterLifecycle.Version,
		},
		API: LifecycleAPI{
			BuildpackVersion: clusterLifecycle.Status.ResolvedClusterLifecycle.API.BuildpackVersion,
			PlatformVersion:  clusterLifecycle.Status.ResolvedClusterLifecycle.API.PlatformVersion,
		},
		APIs: LifecycleAPIs{
			Buildpack: APIVersions{
				Deprecated: toCNBAPISet(clusterLifecycle.Status.ResolvedClusterLifecycle.APIs.Buildpack.Deprecated),
				Supported:  toCNBAPISet(clusterLifecycle.Status.ResolvedClusterLifecycle.APIs.Buildpack.Supported),
			},
			Platform: APIVersions{
				Deprecated: toCNBAPISet(clusterLifecycle.Status.ResolvedClusterLifecycle.APIs.Platform.Deprecated),
				Supported:  toCNBAPISet(clusterLifecycle.Status.ResolvedClusterLifecycle.APIs.Platform.Supported),
			},
		},
	}

	lifecycleLayer, err = func() (ggcrv1.Layer, error) {
		manifest, err := lifecycleImage.Manifest()
		if err != nil || manifest == nil {
			return nil, fmt.Errorf("failed to get manifest file: %w", err)
		}
		if len(manifest.Layers) < 1 {
			return nil, errors.New("failed to find lifecycle image layers")
		}
		lastLayerDigest := manifest.Layers[len(manifest.Layers)-1].Digest
		return lifecycleImage.LayerByDigest(lastLayerDigest)
	}()
	if err != nil {
		return nil, LifecycleMetadata{}, fmt.Errorf("failed to find lifecycle layer: %w", err)
	}

	err = func() error {
		cfg, err := lifecycleImage.ConfigFile()
		if err != nil || cfg == nil {
			return fmt.Errorf("failed to get config file: %w", err)
		}
		if !platformMatches(
			builderBlder.os, builderBlder.arch, builderBlder.archVariant,
			cfg.OS, cfg.Architecture, cfg.Variant,
		) {
			return fmt.Errorf(
				"validating lifecycle image %s: expected platform to be %s/%s/%s but got %s/%s/%s",
				clusterLifecycle.Spec.Image,
				builderBlder.os, builderBlder.arch, builderBlder.archVariant,
				cfg.OS, cfg.Architecture, cfg.Variant,
			)
		}
		return nil
	}()
	if err != nil {
		return nil, LifecycleMetadata{}, err
	}

	return lifecycleLayer, lifecycleMetadata, nil
}

func platformMatches(wantOS, wantArch, wantArchVariant string, gotOS, gotArch, gotArchVariant string) bool {
	if wantOS != gotOS {
		return false
	}
	if wantArch != "" && gotArch != "" && wantArch != gotArch {
		return false
	}
	if wantArchVariant != "" && gotArchVariant != "" && wantArchVariant != gotArchVariant {
		return false
	}
	return true
}

func toCNBAPISet(from buildapi.APISet) APISet {
	var to APISet
	for _, f := range from {
		to = append(to, f)
	}
	return to
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
