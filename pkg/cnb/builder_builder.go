package cnb

import (
	"archive/tar"
	"bytes"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/semver/v3"
	"github.com/buildpacks/imgutil/layer"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	workspaceDir               = "/workspace"
	layersDir                  = "/layers"
	cnbDir                     = "/cnb"
	platformDir                = "/platform"
	platformEnvDir             = platformDir + "/env"
	buildpacksDir              = "/cnb/buildpacks"
	orderTomlPath              = "/cnb/order.toml"
	stackTomlPath              = "/cnb/stack.toml"
	relaxedMixinMinPlatformAPI = "0.7"
)

var (
	normalizedTime        = time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)
	supportedPlatformApis = []string{"0.3", "0.4", "0.5", "0.6", "0.7", "0.8"}
)

type builderBlder struct {
	baseImage         v1.Image
	lifecycleImage    v1.Image
	LifecycleMetadata LifecycleMetadata
	stackId           string
	order             []corev1alpha1.OrderEntry
	buildpackLayers   map[DescriptiveBuildpackInfo]buildpackLayer
	cnbUserId         int
	cnbGroupId        int
	kpackVersion      string
	runImage          string
	mixins            []string
	os                string
}

func newBuilderBldr(lifecycleImage v1.Image, kpackVersion string) (*builderBlder, error) {
	lifecycleMd := LifecycleMetadata{}
	err := imagehelpers.GetLabel(lifecycleImage, lifecycleMetadataLabel, &lifecycleMd)
	if err != nil {
		return nil, err
	}

	return &builderBlder{
		lifecycleImage:    lifecycleImage,
		LifecycleMetadata: lifecycleMd,
		buildpackLayers:   map[DescriptiveBuildpackInfo]buildpackLayer{},
		kpackVersion:      kpackVersion,
	}, nil
}

func (bb *builderBlder) AddStack(baseImage v1.Image, clusterStack *buildapi.ClusterStack) error {
	file, err := baseImage.ConfigFile()
	if err != nil {
		return err
	}

	bb.os = file.OS
	bb.baseImage = baseImage
	bb.stackId = clusterStack.Status.Id
	bb.runImage = clusterStack.Status.RunImage.Image
	bb.mixins = clusterStack.Status.Mixins
	bb.cnbUserId = clusterStack.Status.UserID
	bb.cnbGroupId = clusterStack.Status.GroupID
	return nil
}

func (bb *builderBlder) AddGroup(buildpacks ...RemoteBuildpackRef) {
	group := make([]corev1alpha1.BuildpackRef, 0, len(buildpacks))
	for _, b := range buildpacks {
		group = append(group, b.buildpackRef())

		for _, layer := range b.Layers {
			bb.buildpackLayers[layer.BuildpackInfo] = layer
		}
	}
	bb.order = append(bb.order, corev1alpha1.OrderEntry{Group: group})
}

func (bb *builderBlder) WriteableImage() (v1.Image, error) {
	buildpacks := bb.buildpacks()

	err := bb.validateBuilder(buildpacks)
	if err != nil {
		return nil, err
	}

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

	image, err = imagehelpers.SetStringLabel(image, lifecycleVersionLabel, bb.LifecycleMetadata.Version)
	if err != nil {
		return nil, err
	}

	return imagehelpers.SetLabels(image, map[string]interface{}{
		buildpackOrderLabel:  bb.order,
		buildpackLayersLabel: buildpackLayerMetadata,
		lifecycleApisLabel:   bb.LifecycleMetadata.APIs,
		buildpackMetadataLabel: BuilderImageMetadata{
			Description: "Custom Builder built with kpack",
			Stack: StackMetadata{
				RunImage: RunImageMetadata{
					Image:   bb.runImage,
					Mirrors: nil,
				},
			},
			Lifecycle: bb.LifecycleMetadata,
			CreatedBy: CreatorMetadata{
				Name:    "kpack Builder",
				Version: bb.kpackVersion,
			},
			Buildpacks: buildpacks,
		},
	})
}

func (bb *builderBlder) validateBuilder(sortedBuildpacks []DescriptiveBuildpackInfo) error {
	platformApis := append(bb.LifecycleMetadata.APIs.Platform.Deprecated, bb.LifecycleMetadata.APIs.Platform.Supported...)
	err := validatePlatformApis(platformApis)
	if err != nil {
		return err
	}
	buildpackApis := append(bb.LifecycleMetadata.APIs.Buildpack.Deprecated, bb.LifecycleMetadata.APIs.Buildpack.Supported...)
	for _, bpInfo := range sortedBuildpacks {

		bpLayerInfo := bb.buildpackLayers[bpInfo].BuildpackLayerInfo
		err := bpLayerInfo.supports(buildpackApis, bb.stackId, bb.mixins, relaxedMixinContract(platformApis))
		if err != nil {
			return errors.Wrapf(err, "validating buildpack %s", bpInfo)
		}
	}
	return nil
}

func validatePlatformApis(builderSupportedApis []string) error {
	for _, api := range supportedPlatformApis {
		if present(builderSupportedApis, api) {
			return nil
		}
	}

	return errors.Errorf("unsupported platform apis in kpack lifecycle: %s, expecting one of: %s", strings.Join(builderSupportedApis, ", "), strings.Join(supportedPlatformApis, ", "))
}

func relaxedMixinContract(builderSupportedApis []string) bool {
	for _, api := range builderSupportedApis {
		if semver.MustParse(api).Compare(semver.MustParse(relaxedMixinMinPlatformAPI)) >= 0 {
			return true
		}
	}
	return false
}

func (bb *builderBlder) buildpacks() []DescriptiveBuildpackInfo {
	return deterministicSortBySize(bb.buildpackLayers)
}

func (bb *builderBlder) lifecycleLayer() (v1.Layer, error) {
	diffId, err := imagehelpers.GetStringLabel(bb.lifecycleImage, bb.os)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find lifecycle for os: %s", bb.os)
	}

	hash, err := v1.NewHash(diffId)
	if err != nil {
		return nil, err
	}

	return bb.lifecycleImage.LayerByDiffID(hash)
}

func (bb *builderBlder) stackLayer() (v1.Layer, error) {
	type tomlRunImage struct {
		Image string `toml:"image"`
	}

	type tomlStackFile struct {
		RunImage tomlRunImage `toml:"run-image"`
	}

	stackBuf := &bytes.Buffer{}
	stackFile := tomlStackFile{
		RunImage: tomlRunImage{
			Image: bb.runImage,
		},
	}
	err := toml.NewEncoder(stackBuf).Encode(stackFile)
	if err != nil {
		return nil, err
	}
	return bb.singeFileLayer(stackTomlPath, stackBuf.Bytes())
}

func (bb *builderBlder) orderLayer() (v1.Layer, error) {
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
	return bb.singeFileLayer(orderTomlPath, orderBuf.Bytes())
}

func (bb *builderBlder) singeFileLayer(file string, contents []byte) (v1.Layer, error) {
	b := &bytes.Buffer{}
	w := bb.layerWriter(b)
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

func (bb *builderBlder) defaultDirsLayer() (v1.Layer, error) {
	dirs := []*tar.Header{
		bb.kpackOwnedDir(workspaceDir),
		bb.kpackOwnedDir(layersDir),
		bb.rootOwnedDir(cnbDir),
		bb.rootOwnedDir(buildpacksDir),
		bb.rootOwnedDir(platformDir),
		bb.rootOwnedDir(platformEnvDir),
	}

	b := &bytes.Buffer{}
	tw := bb.layerWriter(b)

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

func (bb *builderBlder) kpackOwnedDir(path string) *tar.Header {
	return &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     path,
		Mode:     0755,
		ModTime:  normalizedTime,
		Uid:      bb.cnbUserId,
		Gid:      bb.cnbGroupId,
	}
}

func (bb *builderBlder) rootOwnedDir(path string) *tar.Header {
	return &tar.Header{
		Typeflag: tar.TypeDir,
		Name:     path,
		Mode:     0755,
		ModTime:  normalizedTime,
	}
}

func (bb *builderBlder) layerWriter(fileWriter io.Writer) layerWriter {
	if bb.os == "windows" {
		return layer.NewWindowsWriter(fileWriter)
	}
	return tar.NewWriter(fileWriter)

}

type layerWriter interface {
	WriteHeader(hdr *tar.Header) error
	Write(b []byte) (int, error)
	Close() error
}

func deterministicSortBySize(layers map[DescriptiveBuildpackInfo]buildpackLayer) []DescriptiveBuildpackInfo {
	keys := make([]DescriptiveBuildpackInfo, 0, len(layers))
	sizes := make(map[DescriptiveBuildpackInfo]int64, len(layers))
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

func layers(layers ...[]v1.Layer) []v1.Layer {
	var appendedLayers []v1.Layer
	for _, l := range layers {
		appendedLayers = append(appendedLayers, l...)
	}
	return appendedLayers
}
