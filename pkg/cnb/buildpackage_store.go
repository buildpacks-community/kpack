package cnb

import (
	"encoding/json"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/pkg/errors"
	"io"
)

type BuildPackageStoreFactory struct {
}

func (f *BuildPackageStoreFactory) MakeStore(keychain authn.Keychain, storeImage string) (Store, error) {
	ref, err := name.ParseReference(storeImage, name.WeakValidation)
	if err != nil {
		return nil, err
	}

	image, err := remote.Image(ref, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, err
	}

	file, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	metadata, ok := file.Config.Labels[buildpackLayersLabel]
	if !ok {
		return nil, errors.Errorf("error reading metadata %s label on store", buildpackLayersLabel)
	}

	packageMetadata := &BuildpackLayerMetadata{}
	err = json.Unmarshal([]byte(metadata), packageMetadata)
	if err != nil {
		return nil, errors.Wrap(err, "parsing store metadata")
	}

	return &BuildPackageStore{PackageMetadata: *packageMetadata, Image: image}, nil
}

type BuildPackageStore struct {
	Image           v1.Image
	PackageMetadata BuildpackLayerMetadata
}

func (b *BuildPackageStore) FetchBuildpack(id, version string) (RemoteBuildpackInfo, error) {
	buildpackInfo, layerInfo, err := b.PackageMetadata.metadataFor(id, version)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	diffID, err := v1.NewHash(layerInfo.LayerDiffID)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	layer, err := b.Image.LayerByDiffID(diffID)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	layers, err := b.layersForOrder(layerInfo.Order)
	if err != nil {
		return RemoteBuildpackInfo{}, err
	}

	return RemoteBuildpackInfo{
		BuildpackInfo: buildpackInfo,
		Layers: append(layers, buildpackLayer{
			v1Layer:       layer,
			BuildpackInfo: buildpackInfo,
			Order:         layerInfo.Order,
		}),
	}, nil
}

func (b *BuildPackageStore) layersForOrder(order Order) ([]buildpackLayer, error) {
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

func kpackmountableLayer(ref name.Reference, digest, diffId string) v1.Layer {
	return &kpackLayer{diffId, digest}

	//return &remote.MountableLayer{
	//	Layer:     &kpackLayer{diffId, digest},
	//	Reference: ref,
	//}
}

type kpackLayer struct {
	diffId string
	digest string
}

func (l *kpackLayer) Compressed() (io.ReadCloser, error) {
	panic("implement me")
}

func (l *kpackLayer) Uncompressed() (io.ReadCloser, error) {
	panic("implement me")
}

func (l *kpackLayer) Size() (int64, error) {
	return 6013008, nil
}

func (l *kpackLayer) MediaType() (types.MediaType, error) {
	return types.DockerLayer, nil
}

func (l *kpackLayer) Digest() (v1.Hash, error) {
	return v1.NewHash(l.digest)
}

func (l *kpackLayer) DiffID() (v1.Hash, error) {
	return v1.NewHash(l.diffId)
}
