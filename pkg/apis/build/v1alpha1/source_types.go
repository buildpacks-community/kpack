package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// +k8s:openapi-gen=true
type SourceConfig struct {
	Git      *Git      `json:"git,omitempty"`
	Blob     *Blob     `json:"blob,omitempty"`
	Registry *Registry `json:"registry,omitempty"`
	SubPath  string    `json:"subPath,omitempty"`
}

type Source interface {
	BuildEnvVars() []corev1.EnvVar
	ImagePullSecretsVolume() corev1.Volume
}

// +k8s:openapi-gen=true
type Git struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
}

// +k8s:openapi-gen=true
type Blob struct {
	URL string `json:"url"`
}

// +k8s:openapi-gen=true
type Registry struct {
	Image string `json:"image"`
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`
}

// +k8s:openapi-gen=true
type ResolvedSourceConfig struct {
	Git      *ResolvedGitSource      `json:"git,omitempty"`
	Blob     *ResolvedBlobSource     `json:"blob,omitempty"`
	Registry *ResolvedRegistrySource `json:"registry,omitempty"`
}

func (sc ResolvedSourceConfig) ResolvedSource() ResolvedSource {
	if sc.Git != nil {
		return sc.Git
	} else if sc.Blob != nil {
		return sc.Blob
	} else if sc.Registry != nil {
		return sc.Registry
	}
	return nil
}

type ResolvedSource interface {
	IsUnknown() bool
	IsPollable() bool
	SourceConfig() SourceConfig
}

type GitSourceKind string

const (
	Unknown GitSourceKind = "Unknown"
	Branch  GitSourceKind = "Branch"
	Tag     GitSourceKind = "Tag"
	Commit  GitSourceKind = "Commit"
)

// +k8s:openapi-gen=true
type ResolvedGitSource struct {
	URL      string        `json:"url"`
	Revision string        `json:"revision"`
	SubPath  string        `json:"subPath,omitempty"`
	Type     GitSourceKind `json:"type"`
}

func (gs *ResolvedGitSource) SourceConfig() SourceConfig {
	return SourceConfig{
		Git: &Git{
			URL:      gs.URL,
			Revision: gs.Revision,
		},
		SubPath: gs.SubPath,
	}
}

func (gs *ResolvedGitSource) IsUnknown() bool {
	return gs.Type == Unknown
}

func (gs *ResolvedGitSource) IsPollable() bool {
	return gs.Type != Commit && gs.Type != Unknown
}

// +k8s:openapi-gen=true
type ResolvedBlobSource struct {
	URL     string `json:"url"`
	SubPath string `json:"subPath,omitempty"`
}

func (bs *ResolvedBlobSource) SourceConfig() SourceConfig {
	return SourceConfig{
		Blob: &Blob{
			URL: bs.URL,
		},
		SubPath: bs.SubPath,
	}
}

func (bs *ResolvedBlobSource) IsUnknown() bool {
	return false
}

func (bs *ResolvedBlobSource) IsPollable() bool {
	return false
}

// +k8s:openapi-gen=true
type ResolvedRegistrySource struct {
	Image   string `json:"image"`
	SubPath string `json:"subPath,omitempty"`
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`
}

func (rs *ResolvedRegistrySource) SourceConfig() SourceConfig {
	return SourceConfig{
		Registry: &Registry{
			Image:            rs.Image,
			ImagePullSecrets: rs.ImagePullSecrets,
		},
		SubPath: rs.SubPath,
	}
}

func (rs *ResolvedRegistrySource) IsUnknown() bool {
	return false
}

func (rs *ResolvedRegistrySource) IsPollable() bool {
	return false
}
