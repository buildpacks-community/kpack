package cnb

import (
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

type BuildpackClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
}

type BuildpackRetriever struct {
	Keychain authn.Keychain
	Client   BuildpackClient
	Store    *v1alpha1.Store
}

type BuildpackNotFound error

func buildpackNotFoundError(msg string) BuildpackNotFound {
	return BuildpackNotFound(errors.New(msg))
}

func (b *BuildpackRetriever) FetchBuildpack(id, version string) (RemoteBuildpackInfo, error) {
	var storeBuildpack *v1alpha1.StoreBuildpack = nil
	for _, buildpack := range b.Store.Status.Buildpacks {
		if buildpack.ID == id {
			if version == "" || buildpack.Version == version {
				storeBuildpack = &buildpack
				break
			}
		}
	}

	if storeBuildpack == nil {
		return RemoteBuildpackInfo{}, buildpackNotFoundError(fmt.Sprintf("could not find buildpack with id '%s' and version '%s'", id, version))
	}

	buildPackageImage, _, err := b.Client.Fetch(b.Keychain, storeBuildpack.BuildPackage.Image)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	diffID, err := v1.NewHash(storeBuildpack.LayerDiffID)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	layer, err := buildPackageImage.LayerByDiffID(diffID)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	layers, err := b.layersForOrder(toCnbOrder(storeBuildpack.Order))
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	info := BuildpackInfo{
		ID:      storeBuildpack.ID,
		Version: storeBuildpack.Version,
	}

	return RemoteBuildpackInfo{
		BuildpackInfo: info,
		Layers: append(layers, buildpackLayer{
			v1Layer:       layer,
			BuildpackInfo: info,
			Order:         toCnbOrder(storeBuildpack.Order),
		}),
	}, nil
}

func toCnbOrder(storeOrder v1alpha1.Order) Order {
	var order Order
	for _, storeEntry := range storeOrder {
		var entry OrderEntry
		for _, storeBuildpack := range storeEntry.Group {
			entry.Group = append(entry.Group, BuildpackRef{
				BuildpackInfo: BuildpackInfo{
					ID:      storeBuildpack.ID,
					Version: storeBuildpack.Version,
				},
				Optional: storeBuildpack.Optional,
			})
		}
		order = append(order, entry)
	}
	return order
}

// TODO: ensure there are no cycles in the buildpack graph
func (b *BuildpackRetriever) layersForOrder(order Order) ([]buildpackLayer, error) {
	var buildpackLayers []buildpackLayer
	for _, orderEntry := range order {
		for _, buildpackRef := range orderEntry.Group {
			buildpackInfo, err := b.FetchBuildpack(buildpackRef.ID, buildpackRef.Version)
			if err != nil {
				return nil, err
			}

			buildpackLayers = append(buildpackLayers, buildpackInfo.Layers...)
		}

	}
	return buildpackLayers, nil
}
