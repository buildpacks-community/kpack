package cnb

import (
	"github.com/pkg/errors"
	"sort"
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

func (l BuildpackLayerMetadata) metadataFor(id, version string) (BuildpackInfo, BuildpackLayerInfo, error) {
	versionMap, ok := l[id]
	if !ok {
		return BuildpackInfo{}, BuildpackLayerInfo{}, errors.Errorf("could not find buildpack: %s", id)
	}

	if version == "" {
		if len(versionMap) == 0 {
			return BuildpackInfo{}, BuildpackLayerInfo{}, errors.Errorf("no versions of buildpack: %s", id)
		}

		version = highestVersion(versionMap)
		return BuildpackInfo{id, version}, versionMap[version], nil
	}

	buildpackLayer, ok := versionMap[version]
	if !ok {
		return BuildpackInfo{}, BuildpackLayerInfo{}, errors.Errorf("could not find buildpack with id: %s and version: %s", id, version)
	}
	return BuildpackInfo{id, version}, buildpackLayer, nil
}

func highestVersion(versionMap map[string]BuildpackLayerInfo) string {
	versions := make([]string, 0, len(versionMap))
	for v := range versionMap {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	return versions[len(versions)-1]
}
