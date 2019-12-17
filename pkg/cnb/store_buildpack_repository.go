package cnb

import (
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

type StoreBuildpackRepository struct {
	Keychain       authn.Keychain
	RegistryClient RegistryClient

	Store *v1alpha1.Store
}

func (s *StoreBuildpackRepository) FindByIdAndVersion(id, version string) (RemoteBuildpackInfo, error) {
	storeBuildpack, err := s.findBuildpack(id, version)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	buildPackageImage, _, err := s.RegistryClient.Fetch(s.Keychain, storeBuildpack.BuildPackage.Image)
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

	layers, err := s.layersForOrder(toCnbOrder(storeBuildpack.Order))
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

func (s *StoreBuildpackRepository) findBuildpack(id, version string) (v1alpha1.StoreBuildpack, error) {
	var matchingBuildpacks []v1alpha1.StoreBuildpack
	for _, buildpack := range s.Store.Status.Buildpacks {
		if buildpack.ID == id {
			matchingBuildpacks = append(matchingBuildpacks, buildpack)
		}
	}

	if len(matchingBuildpacks) == 0 {
		return v1alpha1.StoreBuildpack{}, errors.Errorf("could not find buildpack with id '%s'", id)
	}

	if version == "" {
		return highestVersion(matchingBuildpacks), nil
	}

	for _, buildpack := range matchingBuildpacks {
		if buildpack.Version == version {
			return buildpack, nil
		}
	}

	return v1alpha1.StoreBuildpack{}, errors.Errorf("could not find buildpack with id '%s' and version '%s'", id, version)
}

// TODO: ensure there are no cycles in the buildpack graph
func (s *StoreBuildpackRepository) layersForOrder(order Order) ([]buildpackLayer, error) {
	var buildpackLayers []buildpackLayer
	for _, orderEntry := range order {
		for _, buildpackRef := range orderEntry.Group {
			buildpackInfo, err := s.FindByIdAndVersion(buildpackRef.ID, buildpackRef.Version)
			if err != nil {
				return nil, err
			}

			buildpackLayers = append(buildpackLayers, buildpackInfo.Layers...)
		}

	}
	return buildpackLayers, nil
}

func toCnbOrder(groups []v1alpha1.Group) Order {
	var order Order
	for _, elem := range groups {
		var entry OrderEntry
		for _, buildpack := range elem.Group {
			entry.Group = append(entry.Group, BuildpackRef{
				BuildpackInfo: BuildpackInfo{
					ID:      buildpack.ID,
					Version: buildpack.Version,
				},
				Optional: buildpack.Optional,
			})
		}
		order = append(order, entry)
	}
	return order
}

func highestVersion(matchingBuildpacks []v1alpha1.StoreBuildpack) v1alpha1.StoreBuildpack {
	sort.Sort(byBuildpackVersion(matchingBuildpacks))
	return matchingBuildpacks[len(matchingBuildpacks)-1]
}

type byBuildpackVersion []v1alpha1.StoreBuildpack

func (b byBuildpackVersion) Len() int {
	return len(b)
}

func (b byBuildpackVersion) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byBuildpackVersion) Less(i, j int) bool {
	return b[i].Version < b[j].Version
}
