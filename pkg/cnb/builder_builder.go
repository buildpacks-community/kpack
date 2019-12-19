package cnb

import (
	"archive/tar"
	"bytes"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type BuilderBuilder struct {
	baseImage       v1.Image
	baseMetadata    *BuilderImageMetadata
	order           []OrderEntry
	stackID         string
	buildpackLayers map[BuildpackInfo]buildpackLayer
}

func newBuilderBuilder(baseImage v1.Image) (*BuilderBuilder, error) {
	baseMetadata := &BuilderImageMetadata{}
	err := imagehelpers.GetLabel(baseImage, buildpackMetadataLabel, baseMetadata)
	if err != nil {
		return nil, err
	}

	stackID, err := imagehelpers.GetStringLabel(baseImage, stackMetadataLabel)
	if err != nil {
		return nil, err
	}

	return &BuilderBuilder{
		baseImage:       baseImage,
		baseMetadata:    baseMetadata,
		stackID:         stackID,
		buildpackLayers: map[BuildpackInfo]buildpackLayer{},
	}, nil
}

func (bb *BuilderBuilder) addGroup(buildpacks ...RemoteBuildpackRef) {
	group := make([]BuildpackRef, 0, len(buildpacks))
	for _, b := range buildpacks {
		group = append(group, b.BuildpackRef)

		for _, layer := range b.Layers {
			bb.buildpackLayers[layer.BuildpackInfo] = layer
		}
	}
	bb.order = append(bb.order, OrderEntry{Group: group})
}

func (bb *BuilderBuilder) buildpacks() v1alpha1.BuildpackMetadataList {
	buildpacks := make(v1alpha1.BuildpackMetadataList, 0, len(bb.buildpackLayers))

	for _, bp := range deterministicSortBySize(bb.buildpackLayers) {
		buildpacks = append(buildpacks, v1alpha1.BuildpackMetadata{
			ID:      bp.ID,
			Version: bp.Version,
		})
	}

	return buildpacks
}

func (bb *BuilderBuilder) writeableImage() (v1.Image, error) {
	buildpackLayerMetadata := make(BuildpackLayerMetadata)
	buildpacks := make([]BuildpackInfo, 0, len(bb.buildpackLayers))
	layers := make([]v1.Layer, 0, len(bb.buildpackLayers)+1)

	for _, key := range deterministicSortBySize(bb.buildpackLayers) {
		layer := bb.buildpackLayers[key]
		if err := buildpackLayerMetadata.add(layer); err != nil {
			return nil, err
		}
		buildpacks = append(buildpacks, key)
		layers = append(layers, layer.v1Layer)
	}

	orderLayer, err := bb.tomlLayer()
	if err != nil {
		return nil, err
	}

	image, err := mutate.AppendLayers(bb.baseImage, append(layers, orderLayer)...)
	if err != nil {
		return nil, err
	}

	return imagehelpers.SetLabels(image, map[string]interface{}{
		buildpackOrderLabel:  bb.order,
		buildpackLayersLabel: buildpackLayerMetadata,
		buildpackMetadataLabel: BuilderImageMetadata{
			Description: "Custom Builder built with kpack",
			Stack:       bb.baseMetadata.Stack,
			Lifecycle:   bb.baseMetadata.Lifecycle,
			CreatedBy: CreatorMetadata{
				Name:    "kpack CustomBuilder",
				Version: "",
			},
			Buildpacks: buildpacks,
		},
	})
}

func (bb *BuilderBuilder) tomlLayer() (v1.Layer, error) {
	orderBuf := &bytes.Buffer{}
	err := toml.NewEncoder(orderBuf).Encode(TomlOrder{bb.order})
	if err != nil {
		return nil, err
	}
	return singeFileLayer(orderTomlPath, orderBuf.Bytes())
}

var normalizedTime = time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)

func singeFileLayer(file string, contents []byte) (v1.Layer, error) {
	b := &bytes.Buffer{}
	w := tar.NewWriter(b)
	if err := w.WriteHeader(&tar.Header{
		Name:    file,
		Size:    int64(len(contents)),
		Mode:    0644,
		ModTime: normalizedTime,
	}); err != nil {
		return nil, err
	}

	if _, err := w.Write(contents); err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return tarball.LayerFromReader(b)
}

func deterministicSortBySize(layers map[BuildpackInfo]buildpackLayer) []BuildpackInfo {
	keys := make([]BuildpackInfo, 0, len(layers))
	sizes := make(map[BuildpackInfo]int64, len(layers))
	for k, layer := range layers {
		keys = append(keys, k)
		size, _ := layer.v1Layer.Size()
		sizes[k] = size
	}

	sort.Slice(keys, func(i, j int) bool {
		sizeI := sizes[keys[i]]
		sizeJ := sizes[keys[j]]

		if sizeI == sizeJ {
			return keys[i].String() > keys[j].String()
		}

		return sizeI > sizeJ
	})

	return keys
}
