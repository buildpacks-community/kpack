package cnb

import (
	"archive/tar"
	"bytes"
	"sort"
	"strconv"
	"time"

	"github.com/BurntSushi/toml"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"

	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	workspaceDir        = "/workspace"
	layersDir           = "/layers"
	cnbDir              = "/cnb"
	cnbLifecycleDir     = "/cnb/lifecycle"
	lifecycleSymlinkDir = "/lifecycle"
	platformDir         = "/platform"
	platformEnvDir      = platformDir + "/env"
	buildpacksDir       = "/cnb/buildpacks"
	orderTomlPath       = "/cnb/order.toml"
	stackTomlPath       = "/cnb/stack.toml"

	cnbUserId  = "CNB_USER_ID"
	cnbGroupId = "CNB_GROUP_ID"
)

var normalizedTime = time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)

type BuilderBuilder struct {
	baseImage         v1.Image
	lifecycleImage    v1.Image
	LifecycleMetadata LifecycleMetadata
	stack             *expv1alpha1.Stack
	order             []expv1alpha1.OrderEntry
	buildpackLayers   map[expv1alpha1.BuildpackInfo]buildpackLayer
	cnbUserId         int
	cnbGroupId        int
	kpackVersion      string
}

func newBuilderBuilder(baseImage v1.Image, lifecycleImage v1.Image, stack *expv1alpha1.Stack, kpackVersion string) (*BuilderBuilder, error) {
	lifecycleMd := LifecycleMetadata{}
	err := imagehelpers.GetLabel(lifecycleImage, lifecycleMetadataLabel, &lifecycleMd)
	if err != nil {
		return nil, err
	}

	userId, err := parseCNBID(baseImage, cnbUserId)
	if err != nil {
		return nil, err
	}

	groupId, err := parseCNBID(baseImage, cnbGroupId)
	if err != nil {
		return nil, err
	}

	return &BuilderBuilder{
		baseImage:         baseImage,
		lifecycleImage:    lifecycleImage,
		LifecycleMetadata: lifecycleMd,
		stack:             stack,
		buildpackLayers:   map[expv1alpha1.BuildpackInfo]buildpackLayer{},
		cnbGroupId:        groupId,
		cnbUserId:         userId,
		kpackVersion:      kpackVersion,
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

func (bb *BuilderBuilder) buildpacks() []expv1alpha1.BuildpackInfo {
	return deterministicSortBySize(bb.buildpackLayers)
}

func (bb *BuilderBuilder) writeableImage() (v1.Image, error) {
	buildpacks := bb.buildpacks()

	buildpackLayerMetadata := BuildpackLayerMetadata{}
	buildpackLayers := make([]v1.Layer, 0, len(bb.buildpackLayers))

	for _, key := range buildpacks {
		layer := bb.buildpackLayers[key]
		buildpackLayerMetadata.add(layer)
		buildpackLayers = append(buildpackLayers, layer.v1Layer)
	}

	defaultLayer, err := bb.defaultDirsLayer()
	if err != nil {
		return nil, err
	}

	lifecycleLayer, err := bb.lifecycleLayer()
	if err != nil {
		return nil, err
	}

	compatLayer, err := bb.lifecycleCompatLayer()
	if err != nil {
		return nil, err
	}

	stackLayer, err := bb.stackLayer()
	if err != nil {
		return nil, err
	}

	orderLayer, err := bb.orderLayer()
	if err != nil {
		return nil, err
	}

	image, err := mutate.AppendLayers(bb.baseImage,
		layers(
			[]v1.Layer{
				defaultLayer,
				lifecycleLayer,
				compatLayer,
			},
			buildpackLayers,
			[]v1.Layer{
				stackLayer,
				orderLayer,
			},
		)...)
	if err != nil {
		return nil, err
	}

	image, err = imagehelpers.SetWorkingDir(image, layersDir)
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
				Version: bb.kpackVersion,
			},
			Buildpacks: buildpacks,
		},
	})
}

func (bb *BuilderBuilder) lifecycleLayer() (v1.Layer, error) {
	layers, err := bb.lifecycleImage.Layers()
	if err != nil {
		return nil, err
	}

	if len(layers) != 1 {
		return nil, errors.New("invalid lifecycle image")
	}

	return layers[0], nil
}

func (bb *BuilderBuilder) stackLayer() (v1.Layer, error) {
	type tomlRunImage struct {
		Image string `toml:"image"`
	}

	type tomlStackFile struct {
		RunImage tomlRunImage `toml:"run-image"`
	}

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

func (bb *BuilderBuilder) orderLayer() (v1.Layer, error) {
	type tomlBuildpack struct {
		ID       string `toml:"id"`
		Version  string `toml:"version"`
		Optional bool   `toml:"optional,omitempty"`
	}

	type tomlOrderEntry struct {
		Group []tomlBuildpack `toml:"group"`
	}

	type tomlOrder []tomlOrderEntry

	type tomlOrderFile struct {
		Order tomlOrder `toml:"order"`
	}

	orderBuf := &bytes.Buffer{}

	order := make(tomlOrder, 0, len(bb.order))
	for _, o := range bb.order {
		bps := make([]tomlBuildpack, 0, len(o.Group))
		for _, b := range o.Group {
			bps = append(bps, tomlBuildpack{
				ID:       b.Id,
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

func (bb *BuilderBuilder) defaultDirsLayer() (v1.Layer, error) {
	dirs := []*tar.Header{
		bb.kpackOwnedDir(workspaceDir),
		bb.kpackOwnedDir(layersDir),
		bb.rootOwnedDir(cnbDir),
		bb.rootOwnedDir(buildpacksDir),
		bb.rootOwnedDir(platformDir),
		bb.rootOwnedDir(platformEnvDir),
	}

	b := &bytes.Buffer{}
	tw := tar.NewWriter(b)

	for _, header := range dirs {
		if err := tw.WriteHeader(header); err != nil {
			return nil, errors.Wrapf(err, "creating %s dir in layer", header.Name)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return tarball.LayerFromReader(b)
}

func (bb *BuilderBuilder) kpackOwnedDir(path string) *tar.Header {
	return &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     path,
		Mode:     0755,
		ModTime:  normalizedTime,
		Uid:      bb.cnbUserId,
		Gid:      bb.cnbGroupId,
	}
}

func (bb *BuilderBuilder) rootOwnedDir(path string) *tar.Header {
	return &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     path,
		Mode:     0755,
		ModTime:  normalizedTime,
	}
}

func (bb *BuilderBuilder) lifecycleCompatLayer() (v1.Layer, error) {
	b := &bytes.Buffer{}
	tw := tar.NewWriter(b)

	if err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeSymlink,
		Name:     lifecycleSymlinkDir,
		Linkname: cnbLifecycleDir,
		ModTime:  normalizedTime,
		Mode:     0644,
	}); err != nil {
		return nil, errors.Wrapf(err, "creating %s dir in layer", lifecycleSymlinkDir)
	}
	if err := tw.Close(); err != nil {
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

func parseCNBID(image v1.Image, env string) (int, error) {
	v, err := imagehelpers.GetEnv(image, env)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(v)
}

func layers(layers ...[]v1.Layer) []v1.Layer {
	var appendedLayers []v1.Layer
	for _, l := range layers {
		appendedLayers = append(appendedLayers, l...)
	}
	return appendedLayers
}
