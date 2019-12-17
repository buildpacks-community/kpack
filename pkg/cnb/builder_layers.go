package cnb

import (
	"github.com/pkg/errors"
)

type BuildpackLayerMetadata map[string]map[string]BuildpackLayerInfo

type BuildpackLayerInfo struct {
	LayerDigest string `json:"layerDigest"`
	LayerDiffID string `json:"layerDiffID"`
	Order       Order  `json:"order,omitempty"`
}

func (l BuildpackLayerMetadata) add(layer buildpackLayer) error {
	_, ok := l[layer.BuildpackInfo.ID]
	if !ok {
		l[layer.BuildpackInfo.ID] = map[string]BuildpackLayerInfo{}
	}

	diffId, err := layer.v1Layer.DiffID()
	if err != nil {
		return errors.Wrapf(err, "fetching %s@%s layer diff id", layer.BuildpackInfo.ID, layer.BuildpackInfo.Version)
	}

	digest, err := layer.v1Layer.Digest()
	if err != nil {
		return errors.Wrapf(err, "fetching %s@%s layer digest", layer.BuildpackInfo.ID, layer.BuildpackInfo.Version)
	}

	l[layer.BuildpackInfo.ID][layer.BuildpackInfo.Version] = BuildpackLayerInfo{
		LayerDigest: digest.String(),
		LayerDiffID: diffId.String(),
		Order:       layer.Order,
	}

	return nil
}
