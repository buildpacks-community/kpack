package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

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
	ImagePullSecretsVolume() corev1.Volume
}

type Git struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
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
		homeEnv,
	}
}

func (in *Git) ImagePullSecretsVolume() corev1.Volume {
	return corev1.Volume{
		Name: imagePullSecretsDirName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

type Blob struct {
	URL string `json:"url"`
}

func (b *Blob) ImagePullSecretsVolume() corev1.Volume {
	return corev1.Volume{
		Name: imagePullSecretsDirName,
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
		homeEnv,
	}
}

type Registry struct {
	Image            string   `json:"image"`
	ImagePullSecrets []string `json:"imagePullSecrets"`
}

func (r *Registry) ImagePullSecretsVolume() corev1.Volume {
	if len(r.ImagePullSecrets) > 0 {
		return corev1.Volume{
			Name: imagePullSecretsDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: r.ImagePullSecrets[0],
				},
			},
		}
	} else {
		return corev1.Volume{
			Name: imagePullSecretsDirName,
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
		homeEnv,
	}
}

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
	ConfigChanged(lastBuild *Build) bool
	RevisionChanged(lastBuild *Build) bool
	SourceConfig() SourceConfig
}

type GitSourceKind string

const (
	Unknown GitSourceKind = "Unknown"
	Branch  GitSourceKind = "Branch"
	Tag     GitSourceKind = "Tag"
	Commit  GitSourceKind = "Commit"
)

type ResolvedGitSource struct {
	URL      string        `json:"url"`
	Revision string        `json:"commit"`
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

func (gs *ResolvedGitSource) ConfigChanged(lastBuild *Build) bool {
	if lastBuild.Spec.Source.Git == nil {
		return true
	}

	return gs.URL != lastBuild.Spec.Source.Git.URL ||
		gs.SubPath != lastBuild.Spec.Source.SubPath
}

func (gs *ResolvedGitSource) RevisionChanged(lastBuild *Build) bool {
	if lastBuild.Spec.Source.Git == nil {
		return true
	}

	return gs.Revision != lastBuild.Spec.Source.Git.Revision
}

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

func (bs *ResolvedBlobSource) ConfigChanged(lastBuild *Build) bool {
	if lastBuild.Spec.Source.Blob == nil {
		return true
	}
	return bs.URL != lastBuild.Spec.Source.Blob.URL ||
		bs.SubPath != lastBuild.Spec.Source.SubPath
}

func (bs *ResolvedBlobSource) RevisionChanged(lastBuild *Build) bool {
	return false
}

type ResolvedRegistrySource struct {
	Image            string   `json:"image"`
	ImagePullSecrets []string `json:"imagePullSecrets"`
	SubPath          string   `json:"subPath,omitempty"`
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

func (rs *ResolvedRegistrySource) ConfigChanged(lastBuild *Build) bool {
	if lastBuild.Spec.Source.Registry == nil {
		return true
	}

	return rs.Image != lastBuild.Spec.Source.Registry.Image ||
		!equality.Semantic.DeepEqual(rs.ImagePullSecrets, lastBuild.Spec.Source.Registry.ImagePullSecrets) ||
		rs.SubPath != lastBuild.Spec.Source.SubPath
}

func (rs *ResolvedRegistrySource) RevisionChanged(lastBuild *Build) bool {
	return false
}
