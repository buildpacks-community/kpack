package cnb

type BuildpackLayerMetadata map[string]map[string]BuildpackLayerInfo

func (l BuildpackLayerMetadata) add(layer buildpackLayer) error {
	_, ok := l[layer.BuildpackInfo.Id]
	if !ok {
		l[layer.BuildpackInfo.Id] = map[string]BuildpackLayerInfo{}
	}

	l[layer.BuildpackInfo.Id][layer.BuildpackInfo.Version] = layer.BuildpackLayerInfo
	return nil
}
