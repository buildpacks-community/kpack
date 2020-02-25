package cnb

import (
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type RemoteStoreReader struct {
	RegistryClient RegistryClient
}

func (r *RemoteStoreReader) Read(keychain authn.Keychain, storeImages []v1alpha1.StoreImage) ([]v1alpha1.StoreBuildpack, error) {
	var g errgroup.Group

	c := make(chan v1alpha1.StoreBuildpack)
	for _, storeImage := range storeImages {
		storeImage := storeImage
		g.Go(func() error {
			image, _, err := r.RegistryClient.Fetch(keychain, storeImage.Image)
			if err != nil {
				return err
			}

			layerMetadata := BuildpackLayerMetadata{}
			err = imagehelpers.GetLabel(image, buildpackLayersLabel, &layerMetadata)
			if err != nil {
				return err
			}

			for id := range layerMetadata {
				for version, metadata := range layerMetadata[id] {
					info := v1alpha1.BuildpackInfo{
						Id:      id,
						Version: version,
					}

					diffId, err := v1.NewHash(metadata.LayerDiffID)
					if err != nil {
						return errors.Wrapf(err, "unable to parse layer diffId for %s", info)
					}
					layer, err := image.LayerByDiffID(diffId)
					if err != nil {
						return errors.Wrapf(err, "unable to get layer %s", info)
					}

					size, err := layer.Size()
					if err != nil {
						return errors.Wrapf(err, "unable to get layer %s size", info)
					}

					digest, err := layer.Digest()
					if err != nil {
						return errors.Wrapf(err, "unable to get layer %s digest", info)
					}

					c <- v1alpha1.StoreBuildpack{
						BuildpackInfo: info,
						StoreImage:    storeImage,
						Order:         metadata.Order,
						DiffId:        metadata.LayerDiffID,
						Digest:        digest.String(),
						Size:          size,
						API:           metadata.API,
						Stacks:        metadata.Stacks,
					}
				}
			}
			return nil
		})
	}
	go func() {
		_ = g.Wait()
		close(c)
	}()

	var buildpacks []v1alpha1.StoreBuildpack
	for b := range c {
		buildpacks = append(buildpacks, b)
	}

	sort.Slice(buildpacks, func(i, j int) bool {
		return buildpacks[i].String() < buildpacks[j].String()
	})

	return buildpacks, g.Wait()
}
