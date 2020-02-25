package cnb

import (
	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

const (
	buildpackOrderLabel    = "io.buildpacks.buildpack.order"
	buildpackLayersLabel   = "io.buildpacks.buildpack.layers"
	buildpackMetadataLabel = "io.buildpacks.builder.metadata"
	lifecycleMetadataLabel = "io.buildpacks.lifecycle.metadata"
)

type BuildpackLayerInfo struct {
	API         string                       `json:"api"`
	LayerDiffID string                       `json:"layerDiffID"`
	Order       expv1alpha1.Order            `json:"order,omitempty"`
	Stacks      []expv1alpha1.BuildpackStack `json:"stacks,omitempty"`
}

type Stack struct {
	ID     string   `json:"id"`
	Mixins []string `json:"mixins,omitempty"`
}

type BuilderImageMetadata struct {
	Description string                   `json:"description"`
	Stack       StackMetadata            `json:"stack"`
	Lifecycle   LifecycleMetadata        `json:"lifecycle"`
	CreatedBy   CreatorMetadata          `json:"createdBy"`
	Buildpacks  []v1alpha1.BuildpackInfo `json:"buildpacks"`
}

type StackMetadata struct {
	RunImage RunImageMetadata `json:"runImage" toml:"run-image"`
}

type RunImageMetadata struct {
	Image   string   `json:"image" toml:"image"`
	Mirrors []string `json:"mirrors" toml:"mirrors"`
}

type CreatorMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type LifecycleMetadata struct {
	LifecycleInfo
	API LifecycleAPI `json:"api,omitempty"`
}

type LifecycleDescriptor struct {
	Info LifecycleInfo `toml:"lifecycle"`
	API  LifecycleAPI  `toml:"api"`
}

type LifecycleInfo struct {
	Version string `toml:"version" json:"version"`
}

type LifecycleAPI struct {
	BuildpackVersion string `toml:"buildpack" json:"buildpack,omitempty"`
	PlatformVersion  string `toml:"platform" json:"platform,omitempty"`
}

type BuiltImageStack struct {
	RunImage string
	ID       string
}
