package cnb

import (
	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

const (
	buildpackOrderLabel    = "io.buildpacks.buildpack.order"
	buildpackLayersLabel   = "io.buildpacks.buildpack.layers"
	buildpackMetadataLabel = "io.buildpacks.builder.metadata"
	stackMetadataLabel     = "io.buildpacks.stack.id"

	orderTomlPath = "/cnb/order.toml"
)

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

type Stack struct {
	RunImage string
	ID       string
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
