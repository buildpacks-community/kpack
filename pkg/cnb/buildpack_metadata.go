package cnb

import (
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	buildpackOrderLabel    = "io.buildpacks.buildpack.order"
	buildpackLayersLabel   = "io.buildpacks.buildpack.layers"
	buildpackMetadataLabel = "io.buildpacks.builder.metadata"
	lifecycleVersionLabel  = "io.buildpacks.lifecycle.version"
	lifecycleApisLabel     = "io.buildpacks.lifecycle.apis"
)

type BuildpackLayerInfo struct {
	API         string                        `json:"api"`
	LayerDiffID string                        `json:"layerDiffID"`
	Order       corev1alpha1.Order            `json:"order,omitempty"`
	Stacks      []corev1alpha1.BuildpackStack `json:"stacks,omitempty"`
	Homepage    string                        `json:"homepage,omitempty"`
}

type DescriptiveBuildpackInfo struct {
	corev1alpha1.BuildpackInfo
	Homepage string `json:"homepage,omitempty"`
}

type Stack struct {
	ID     string   `json:"id"`
	Mixins []string `json:"mixins,omitempty"`
}

type BuilderImageMetadata struct {
	Description string                     `json:"description"`
	Stack       StackMetadata              `json:"stack"`
	Lifecycle   LifecycleMetadata          `json:"lifecycle"`
	CreatedBy   CreatorMetadata            `json:"createdBy"`
	Buildpacks  []DescriptiveBuildpackInfo `json:"buildpacks"`
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

	// Deprecated: Use `LifecycleAPIs` instead
	API  LifecycleAPI  `json:"api,omitempty"`
	APIs LifecycleAPIs `json:"apis,omitempty"`
}

type LifecycleDescriptor struct {
	Info LifecycleInfo `toml:"lifecycle"`

	// Deprecated: Use `LifecycleAPIs` instead
	API  LifecycleAPI  `toml:"api" json:"api,omitempty"`
	APIs LifecycleAPIs `toml:"apis" json:"apis,omitempty"`
}

type LifecycleInfo struct {
	Version string `toml:"version" json:"version"`
}

type LifecycleAPI struct {
	BuildpackVersion string `toml:"buildpack" json:"buildpack,omitempty"`
	PlatformVersion  string `toml:"platform" json:"platform,omitempty"`
}

type LifecycleAPIs struct {
	Buildpack APIVersions `toml:"buildpack" json:"buildpack"`
	Platform  APIVersions `toml:"platform" json:"platform"`
}

type APIVersions struct {
	Deprecated APISet `toml:"deprecated" json:"deprecated"`
	Supported  APISet `toml:"supported" json:"supported"`
}

type APISet []string

type builtImageStack struct {
	RunImage string
	ID       string
}
