package v1alpha1

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type SourceConfig struct {
	Git      *Git      `json:"git,omitempty"`
	Blob     *Blob     `json:"blob,omitempty"`
	Registry *Registry `json:"registry,omitempty"`
	SubPath  string    `json:"subPath,omitempty"`
}

func (sc *SourceConfig) Source() Source {
	if sc.Git != nil {
		return sc.Git
	} else if sc.Blob != nil {
		return sc.Blob
	} else if sc.Registry != nil {
		return sc.Registry
	}
	return nil
}

type Source interface {
	BuildEnvVars() []corev1.EnvVar
	ImagePullSecretsVolume(name string) corev1.Volume
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type Git struct {
	URL                  string `json:"url"`
	Revision             string `json:"revision"`
	InitializeSubmodules bool   `json:"initializeSubmodules,omitempty"`
}

func (g *Git) BuildEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "GIT_URL",
			Value: g.URL,
		},
		{
			Name:  "GIT_REVISION",
			Value: g.Revision,
		},
		{
			Name:  "GIT_INITIALIZE_SUBMODULES",
			Value: strconv.FormatBool(g.InitializeSubmodules),
		},
	}
}

func (in *Git) ImagePullSecretsVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

type BlobAuthKind string

const (
	BlobAuthNone   BlobAuthKind = ""
	BlobAuthHelper BlobAuthKind = "helper"
	BlobAuthSecret BlobAuthKind = "secret"
)

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type Blob struct {
	URL             string `json:"url"`
	Auth            string `json:"auth,omitempty"`
	StripComponents int64  `json:"stripComponents,omitempty"`
}

func (b *Blob) ImagePullSecretsVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func (b *Blob) BuildEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "BLOB_URL",
			Value: b.URL,
		},
		{
			Name:  "BLOB_STRIP_COMPONENTS",
			Value: strconv.FormatInt(b.StripComponents, 10),
		},
		{
			Name:  "BLOB_AUTH",
			Value: strconv.FormatBool(b.Auth != string(BlobAuthNone)),
		},
	}
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type Registry struct {
	Image string `json:"image"`
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`
}

func (r *Registry) ImagePullSecretsVolume(name string) corev1.Volume {
	if len(r.ImagePullSecrets) > 0 {
		return corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: r.ImagePullSecrets[0].Name,
				},
			},
		}
	} else {
		return corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
}

func (r *Registry) BuildEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "REGISTRY_IMAGE",
			Value: r.Image,
		},
	}
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
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
// +k8s:deepcopy-gen=true
type ResolvedGitSource struct {
	URL                  string        `json:"url"`
	Revision             string        `json:"revision"`
	SubPath              string        `json:"subPath,omitempty"`
	Type                 GitSourceKind `json:"type"`
	InitializeSubmodules bool          `json:"initializeSubmodules,omitempty"`
}

func (gs *ResolvedGitSource) SourceConfig() SourceConfig {
	return SourceConfig{
		Git: &Git{
			URL:                  gs.URL,
			Revision:             gs.Revision,
			InitializeSubmodules: gs.InitializeSubmodules,
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
// +k8s:deepcopy-gen=true
type ResolvedBlobSource struct {
	URL             string `json:"url"`
	Auth            string `json:"auth,omitempty"`
	SubPath         string `json:"subPath,omitempty"`
	StripComponents int64  `json:"stripComponents,omitempty"`
}

func (bs *ResolvedBlobSource) SourceConfig() SourceConfig {
	return SourceConfig{
		Blob: &Blob{
			URL:             bs.URL,
			Auth:            bs.Auth,
			StripComponents: bs.StripComponents,
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
// +k8s:deepcopy-gen=true
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
