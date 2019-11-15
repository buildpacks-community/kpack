package cnb

import "fmt"

const (
	buildpackOrderLabel    = "io.buildpacks.buildpack.order"
	buildpackLayersLabel   = "io.buildpacks.buildpack.layers"
	buildpackMetadataLabel = "io.buildpacks.builder.metadata"
	stackMetadataLabel     = "io.buildpacks.stack.id"

	orderTomlPath = "/cnb/order.toml"
)

type Order []OrderEntry

type OrderEntry struct {
	Group []BuildpackRef `toml:"group" json:"group"`
}

type TomlOrder struct {
	Order Order `toml:"order"`
}

type BuildpackRef struct {
	BuildpackInfo
	Optional bool `toml:"optional,omitempty" json:"optional,omitempty"`
}

type BuildpackInfo struct {
	ID      string `toml:"id" json:"id"`
	Version string `toml:"version" json:"version,omitempty"`
}

func (b BuildpackInfo) String() string {
	return fmt.Sprintf("%s@%s", b.ID, b.Version)
}

type BuilderImageMetadata struct {
	Description string            `json:"description"`
	Stack       StackMetadata     `json:"stack"`
	Lifecycle   LifecycleMetadata `json:"lifecycle"`
	CreatedBy   CreatorMetadata   `json:"createdBy"`
	Buildpacks  []BuildpackInfo   `json:"buildpacks"`
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
