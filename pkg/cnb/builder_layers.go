package cnb

const (
	BuilderMetadataLabel = "io.buildpacks.builder.metadata"
)

type BuildpackLayerMetadata map[string]map[string]BuildpackLayerInfo

func (l BuildpackLayerMetadata) add(layer buildpackLayer) {
	_, ok := l[layer.BuildpackInfo.Id]
	if !ok {
		l[layer.BuildpackInfo.Id] = map[string]BuildpackLayerInfo{}
	}

	l[layer.BuildpackInfo.Id][layer.BuildpackInfo.Version] = layer.BuildpackLayerInfo
}
