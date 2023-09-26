package cnb

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type RemoteBuildpackFetcher interface {
	BuildpackResolver
	ResolveAndFetch(context.Context, buildapi.BuilderBuildpackRef) (RemoteBuildpackInfo, error)
}

type remoteBuildpackFetcher struct {
	BuildpackResolver
	keychainFactory registry.KeychainFactory
}

func NewRemoteBuildpackFetcher(
	factory registry.KeychainFactory,
	clusterStore *buildapi.ClusterStore,
	buildpacks []*buildapi.Buildpack, clusterBuildpacks []*buildapi.ClusterBuildpack,
) RemoteBuildpackFetcher {
	return &remoteBuildpackFetcher{
		BuildpackResolver: NewBuildpackResolver(clusterStore, buildpacks, clusterBuildpacks),
		keychainFactory:   dockercreds.NewCachedKeychainFactory(factory),
	}
}

func (s *remoteBuildpackFetcher) ResolveAndFetch(ctx context.Context, ref buildapi.BuilderBuildpackRef) (RemoteBuildpackInfo, error) {
	remote, err := s.resolve(ref)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	return s.fetch(ctx, remote)
}

// TODO: see if this needs to be updated for extensions
func (s *remoteBuildpackFetcher) fetch(ctx context.Context, remoteBuildpack K8sRemoteBuildpack) (RemoteBuildpackInfo, error) {
	buildpack := remoteBuildpack.Buildpack
	keychain, err := s.keychainFactory.KeychainForSecretRef(ctx, remoteBuildpack.SecretRef)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	layer, err := layerForBuildpack(keychain, buildpack)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	layers, err := s.layersForOrder(ctx, buildpack.Order)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	info := DescriptiveBuildpackInfo{
		BuildpackInfo: corev1alpha1.BuildpackInfo{
			Id:      buildpack.Id,
			Version: buildpack.Version,
		},
		Homepage: buildpack.Homepage,
	}

	return RemoteBuildpackInfo{
		BuildpackInfo: info,
		Layers: append(layers, buildpackLayer{
			v1Layer:       layer,
			BuildpackInfo: info,
			BuildpackLayerInfo: BuildpackLayerInfo{
				LayerDiffID: buildpack.DiffId,
				Order:       buildpack.Order,
				API:         buildpack.API,
				Stacks:      buildpack.Stacks,
				Homepage:    buildpack.Homepage,
			},
		}),
	}, nil
}

// TODO: ensure there are no cycles in the buildpack graph
func (s *remoteBuildpackFetcher) layersForOrder(ctx context.Context, order corev1alpha1.Order) ([]buildpackLayer, error) {
	var buildpackLayers []buildpackLayer
	for _, orderEntry := range order {
		for _, buildpackRef := range orderEntry.Group {
			buildpack, err := s.resolve(buildapi.BuilderBuildpackRef{
				BuildpackRef: corev1alpha1.BuildpackRef{
					BuildpackInfo: corev1alpha1.BuildpackInfo{
						Id:      buildpackRef.Id,
						Version: buildpackRef.Version,
					},
				},
			})
			if err != nil {
				return nil, err
			}

			buildpackInfo, err := s.fetch(ctx, buildpack)
			if err != nil {
				return nil, err
			}

			buildpackLayers = append(buildpackLayers, buildpackInfo.Layers...)
		}
	}
	return buildpackLayers, nil
}

func layerForBuildpack(keychain authn.Keychain, buildpack corev1alpha1.BuildpackStatus) (v1.Layer, error) {
	return imagehelpers.NewLazyMountableLayer(imagehelpers.LazyMountableLayerArgs{
		Digest:   buildpack.Digest,
		DiffId:   buildpack.DiffId,
		Image:    buildpack.StoreImage.Image,
		Size:     buildpack.Size,
		Keychain: keychain,
	})
}
