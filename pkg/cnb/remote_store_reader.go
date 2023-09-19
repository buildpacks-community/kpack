package cnb

import (
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type RemoteBuildpackReader struct {
	RegistryClient RegistryClient
}

func (r *RemoteBuildpackReader) Read(keychain authn.Keychain, storeImages []corev1alpha1.ImageSource) ([]corev1alpha1.BuildpackStatus, error) {
	var g errgroup.Group

	c := make(chan corev1alpha1.BuildpackStatus)
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
				if err2 := imagehelpers.GetLabel(image, "io.buildpacks.extension.layers", &layerMetadata); err2 != nil {
					return err
				}
			}

			for id := range layerMetadata {
				for version, metadata := range layerMetadata[id] {
					packageInfo := corev1alpha1.BuildpackageInfo{
						Id:       bpMetadata.Id,
						Version:  bpMetadata.Version,
						Homepage: bpMetadata.Homepage,
					}

					info := corev1alpha1.BuildpackInfo{
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

					c <- corev1alpha1.BuildpackStatus{
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

	var buildpacks []corev1alpha1.BuildpackStatus
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
