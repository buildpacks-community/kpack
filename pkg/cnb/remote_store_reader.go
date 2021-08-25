package cnb

import (
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type RemoteStoreReader struct {
	RegistryClient RegistryClient
}

func (r *RemoteStoreReader) Read(keychain authn.Keychain, storeImages []buildapi.StoreImage) ([]buildapi.StoreBuildpack, error) {
	var g errgroup.Group

	c := make(chan buildapi.StoreBuildpack)
	for _, storeImage := range storeImages {
		storeImageCopy := storeImage
		g.Go(func() error {
			image, _, err := r.RegistryClient.Fetch(keychain, storeImageCopy.Image)
			if err != nil {
				return err
			}

			bpMetadata := BuildpackageMetadata{}
			if ok, err := imagehelpers.HasLabel(image, buildpackageMetadataLabel); err != nil {
				return err
			} else if ok {
				err := imagehelpers.GetLabel(image, buildpackageMetadataLabel, &bpMetadata)
				if err != nil {
					return err
				}
			}

			layerMetadata := BuildpackLayerMetadata{}
			err = imagehelpers.GetLabel(image, buildpackLayersLabel, &layerMetadata)
			if err != nil {
				return err
			}

			for id := range layerMetadata {
				for version, metadata := range layerMetadata[id] {
					packageInfo := buildapi.BuildpackageInfo{
						Id:       bpMetadata.Id,
						Version:  bpMetadata.Version,
						Homepage: bpMetadata.Homepage,
					}

					info := buildapi.BuildpackInfo{
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

					c <- buildapi.StoreBuildpack{
						BuildpackInfo: info,
						Buildpackage:  packageInfo,
						StoreImage:    storeImageCopy,
						Digest:        digest.String(),
						DiffId:        metadata.LayerDiffID,
						Size:          size,

						Order:    metadata.Order,
						Homepage: metadata.Homepage,
						API:      metadata.API,
						Stacks:   metadata.Stacks,
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

	var buildpacks []buildapi.StoreBuildpack
	for b := range c {
		buildpacks = append(buildpacks, b)
	}

	sort.Slice(buildpacks, func(i, j int) bool {
		if buildpacks[i].String() == buildpacks[j].String() {
			return buildpacks[i].StoreImage.Image < buildpacks[j].StoreImage.Image
		}

		return buildpacks[i].String() < buildpacks[j].String()
	})

	return buildpacks, g.Wait()
}
