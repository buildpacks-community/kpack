package cnb

import (
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type RemoteStoreReader struct {
	RegistryClient RegistryClient
}

func (r *RemoteStoreReader) Read(keychain authn.Keychain, storeImages []v1alpha1.StoreImage) ([]v1alpha1.StoreBuildpack, error) {
	var buildpacks []v1alpha1.StoreBuildpack
	for _, storeImage := range storeImages {
		image, _, err := r.RegistryClient.Fetch(keychain, storeImage.Image)
		if err != nil {
			return nil, err
		}

		layerMetadata := BuildpackLayerMetadata{}
		err = imagehelpers.GetLabel(image, buildpackLayersLabel, &layerMetadata)
		if err != nil {
			return nil, err
		}

		for id := range layerMetadata {
			for version, metadata := range layerMetadata[id] {
				order := metadata.Order

				storeBP := v1alpha1.StoreBuildpack{
					BuildpackInfo: v1alpha1.BuildpackInfo{
						ID:      id,
						Version: version,
					},
					LayerDiffID: metadata.LayerDiffID,
					StoreImage:  storeImage,
					Order:       order,
				}
				buildpacks = append(buildpacks, storeBP)
			}
		}
	}

	sort.Slice(buildpacks, func(i, j int) bool {
		return buildpacks[i].String() < buildpacks[j].String()
	})

	return buildpacks, nil
}
