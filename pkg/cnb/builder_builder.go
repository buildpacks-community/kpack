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
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type BuilderBuilder struct {
	baseImage         v1.Image
	lifecycleImage    v1.Image
	LifecycleMetadata LifecycleMetadata
	stack             *expv1alpha1.Stack
	order             []expv1alpha1.OrderEntry
	buildpackLayers   map[expv1alpha1.BuildpackInfo]buildpackLayer
}

func newBuilderBuilder(baseImage v1.Image, lifecycleImage v1.Image, stack *expv1alpha1.Stack) (*BuilderBuilder, error) {
	lifecycleMd := LifecycleMetadata{}
	err := imagehelpers.GetLabel(lifecycleImage, lifecycleMetadataLabel, &lifecycleMd)
	if err != nil {
		return nil, err
	}

	return &BuilderBuilder{
		baseImage:         baseImage,
		lifecycleImage:    lifecycleImage,
		LifecycleMetadata: lifecycleMd,
		stack:             stack,
		buildpackLayers:   map[expv1alpha1.BuildpackInfo]buildpackLayer{},
	}, nil
}

func (bb *BuilderBuilder) addGroup(buildpacks ...RemoteBuildpackRef) {
	group := make([]expv1alpha1.BuildpackRef, 0, len(buildpacks))
	for _, b := range buildpacks {
		group = append(group, b.BuildpackRef)

		for _, layer := range b.Layers {
			bb.buildpackLayers[layer.BuildpackInfo] = layer
		}
	}
	bb.order = append(bb.order, expv1alpha1.OrderEntry{Group: group})
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
	buildpacks := make([]expv1alpha1.BuildpackInfo, 0, len(bb.buildpackLayers))
	layers := make([]v1.Layer, 0, len(bb.buildpackLayers)+1)

	for _, key := range deterministicSortBySize(bb.buildpackLayers) {
		layer := bb.buildpackLayers[key]
		if err := buildpackLayerMetadata.add(layer); err != nil {
			return nil, err
		}
		buildpacks = append(buildpacks, key)
		layers = append(layers, layer.v1Layer)
	}

	stackLayer, err := bb.stackLayer()
	if err != nil {
		return nil, err
	}

	orderLayer, err := bb.orderLayer()
	if err != nil {
		return nil, err
	}

	lifecycleLayers, err := bb.lifecycleImage.Layers()
	if err != nil {
		return nil, err
	}

	image, err := mutate.AppendLayers(bb.baseImage, append(lifecycleLayers, stackLayer)...)
	if err != nil {
		return nil, err
	}

	image, err = mutate.AppendLayers(image, append(layers, orderLayer)...)
	if err != nil {
		return nil, err
	}

	return imagehelpers.SetLabels(image, map[string]interface{}{
		buildpackOrderLabel:  bb.order,
		buildpackLayersLabel: buildpackLayerMetadata,
		buildpackMetadataLabel: BuilderImageMetadata{
			Description: "Custom Builder built with kpack",
			Stack: StackMetadata{
				RunImage: RunImageMetadata{
					Image:   bb.stack.Spec.RunImage.Image,
					Mirrors: nil,
				},
			},
			Lifecycle: bb.LifecycleMetadata,
			CreatedBy: CreatorMetadata{
				Name:    "kpack CustomBuilder",
				Version: "",
			},
			Buildpacks: buildpacks,
		},
	})
}

func (bb *BuilderBuilder) stackLayer() (v1.Layer, error) {
	stackBuf := &bytes.Buffer{}
	stackFile := tomlStackFile{
		RunImage: tomlRunImage{
			Image: bb.stack.Spec.RunImage.Image,
		},
	}
	err := toml.NewEncoder(stackBuf).Encode(stackFile)
	if err != nil {
		return nil, err
	}
	return singeFileLayer(stackTomlPath, stackBuf.Bytes())
}

type tomlStackFile struct {
	RunImage tomlRunImage `toml:"run-image"`
}

type tomlRunImage struct {
	Image string `toml:"image"`
}

func (bb *BuilderBuilder) orderLayer() (v1.Layer, error) {
	orderBuf := &bytes.Buffer{}

	order := make(tomlOrder, 0, len(bb.order))
	for _, o := range bb.order {
		bps := make([]tomlBuildpack, 0, len(o.Group))
		for _, b := range o.Group {
			bps = append(bps, tomlBuildpack{
				ID:       b.ID,
				Version:  b.Version,
				Optional: b.Optional,
			})
		}
		order = append(order, tomlOrderEntry{Group: bps})
	}

	err := toml.NewEncoder(orderBuf).Encode(tomlOrderFile{order})
	if err != nil {
		return nil, err
	}
	return singeFileLayer(orderTomlPath, orderBuf.Bytes())
}

type tomlOrder []tomlOrderEntry

type tomlOrderEntry struct {
	Group []tomlBuildpack `toml:"group"`
}

type tomlBuildpack struct {
	ID       string `toml:"id"`
	Version  string `toml:"version"`
	Optional bool   `toml:"optional,omitempty"`
}

type tomlOrderFile struct {
	Order tomlOrder `toml:"order"`
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

func deterministicSortBySize(layers map[expv1alpha1.BuildpackInfo]buildpackLayer) []expv1alpha1.BuildpackInfo {
	keys := make([]expv1alpha1.BuildpackInfo, 0, len(layers))
	sizes := make(map[expv1alpha1.BuildpackInfo]int64, len(layers))
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
