package cnb

import (
	"archive/tar"
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
	"sort"
	"time"
)

func newBuilderBuilder(baseImage v1.Image) (*BuilderBuilder, error) {
	baseMetadata := &BuilderImageMetadata{}
	err := registry.GetLabel(baseImage, buildpackMetadataLabel, baseMetadata)
	if err != nil {
		return nil, err
	}

	stackID, err := registry.GetStringLabel(baseImage, stackMetadataLabel)
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

type BuilderBuilder struct {
	baseImage       v1.Image
	baseMetadata    *BuilderImageMetadata
	order           []OrderEntry
	stackID         string
	buildpackLayers map[BuildpackInfo]buildpackLayer
}

func (cb *BuilderBuilder) addGroup(buildpacks ...RemoteBuildpackRef) {
	group := make([]BuildpackRef, 0, len(buildpacks))
	for _, b := range buildpacks {
		group = append(group, b.BuildpackRef)

		for _, layer := range b.Layers {
			cb.buildpackLayers[layer.BuildpackInfo] = layer
		}
	}
	cb.order = append(cb.order, OrderEntry{Group: group})
}

func (cb *BuilderBuilder) stack() v1alpha1.BuildStack {
	return v1alpha1.BuildStack{
		RunImage: cb.baseMetadata.Stack.RunImage.Image,
		ID:       cb.stackID,
	}
}

func (cb *BuilderBuilder) buildpacks() v1alpha1.BuildpackMetadataList {
	buildpacks := make(v1alpha1.BuildpackMetadataList, 0, len(cb.buildpackLayers))

	for _, bp := range deterministicSortBySize(cb.buildpackLayers) {
		buildpacks = append(buildpacks, v1alpha1.BuildpackMetadata{
			ID:      bp.ID,
			Version: bp.Version,
		})
	}

	return buildpacks
}

func (cb *BuilderBuilder) writeableImage() (v1.Image, error) {
	buildpackLayerMetadata := make(BuildpackLayerMetadata)
	buildpacks := make([]BuildpackInfo, 0, len(cb.buildpackLayers))
	layers := make([]v1.Layer, 0, len(cb.buildpackLayers)+1)

	for _, key := range deterministicSortBySize(cb.buildpackLayers) {
		layer := cb.buildpackLayers[key]

		if err := buildpackLayerMetadata.add(layer); err != nil {
			return nil, err
		}

		size, _ := layer.v1Layer.Size()
		var i float64 = float64(size / 1000000)
		fmt.Printf("adding: %s size: %f \n", key, i)

		buildpacks = append(buildpacks, key)
		layers = append(layers, layer.v1Layer)
	}

	orderLayer, err := cb.tomlLayer()
	if err != nil {
		return nil, err
	}

	image, err := mutate.AppendLayers(cb.baseImage, append(layers, orderLayer)...)
	if err != nil {
		return nil, err
	}

	return registry.SetLabels(image, map[string]interface{}{
		buildpackOrderLabel:  cb.order,
		buildpackLayersLabel: buildpackLayerMetadata,
		buildpackMetadataLabel: BuilderImageMetadata{
			Description: "Custom Builder built with kpack",
			Stack:       cb.baseMetadata.Stack,
			Lifecycle:   cb.baseMetadata.Lifecycle,
			CreatedBy: CreatorMetadata{
				Name:    "kpack CustomBuilder",
				Version: "",
			},
			Buildpacks: buildpacks,
		},
	})
}

func (cb *BuilderBuilder) tomlLayer() (v1.Layer, error) {
	orderBuf := &bytes.Buffer{}
	err := toml.NewEncoder(orderBuf).Encode(TomlOrder{cb.order})
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
