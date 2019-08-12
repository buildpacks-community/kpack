package v1alpha1

import (
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

type SourceConfig struct {
	Git      *Git      `json:"git,omitempty"`
	Blob     *Blob     `json:"blob,omitempty"`
	Registry *Registry `json:"registry,omitempty"`
}

type Git struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
}

type Blob struct {
	URL string `json:"url"`
}

type Registry struct {
	Image            string   `json:"image"`
	ImagePullSecrets []string `json:"imagePullSecrets"`
}

type ResolvedSourceConfig struct {
	Git      *ResolvedGitSource      `json:"git,omitempty"`
	Blob     *ResolvedBlobSource     `json:"blob,omitempty"`
	Registry *ResolvedRegistrySource `json:"registry,omitempty"`
}

func (r ResolvedSourceConfig) ResolvedSource() ResolvedSource {
	if r.Git != nil {
		return r.Git
	} else if r.Blob != nil {
		return r.Blob
	}
	return r.Registry
}

type ResolvedSource interface {
	IsUnknown() bool
	IsPollable() bool
	ConfigChanged(lastBuild *Build) bool
	RevisionChanged(lastBuild *Build) bool
	BuildEnvVars() []corev1.EnvVar
	ImagePullSecretsVolume() corev1.Volume
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
	Type     GitSourceKind `json:"type"`
}

func (gs *ResolvedGitSource) ImagePullSecretsVolume() corev1.Volume {
	return corev1.Volume{
		Name: imagePullSecretsDirName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func (gs *ResolvedGitSource) BuildEnvVars() []v1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "GIT_URL",
			Value: gs.URL,
		},
		{
			Name:  "GIT_REVISION",
			Value: gs.Revision,
		},
		homeEnv,
	}
}

func (gs *ResolvedGitSource) IsUnknown() bool {
	return gs.Type == Unknown
}

func (gs *ResolvedGitSource) IsPollable() bool {
	return gs.Type != Commit && gs.Type != Unknown
}

func (gs *ResolvedGitSource) ConfigChanged(lastBuild *Build) bool {
	// return gs != nil && gs.URL != lastBuild.Spec.Source.Git.URL
	if gs == nil {
		return false
	}
	if lastBuild.Spec.Source.Git != nil {
		return gs.URL != lastBuild.Spec.Source.Git.URL
	}
	return true
}

func (gs *ResolvedGitSource) RevisionChanged(lastBuild *Build) bool {
	// return gs != nil && gs.Revision != lastBuild.Spec.Source.Git.Revision
	if gs == nil {
		return false
	}
	if lastBuild.Spec.Source.Git != nil {
		return gs.Revision != lastBuild.Spec.Source.Git.Revision
	}
	return true
}

type ResolvedBlobSource struct {
	URL string `json:"url"`
}

func (bs *ResolvedBlobSource) ImagePullSecretsVolume() corev1.Volume {
	return corev1.Volume{
		Name: imagePullSecretsDirName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func (bs *ResolvedBlobSource) BuildEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "BLOB_URL",
			Value: bs.URL,
		},
		homeEnv,
	}
}

func (bs *ResolvedBlobSource) IsUnknown() bool {
	return false
}

func (bs *ResolvedBlobSource) IsPollable() bool {
	return false
}

func (bs *ResolvedBlobSource) ConfigChanged(lastBuild *Build) bool {
	// return bs != nil && bs.URL != lastBuild.Spec.Source.Blob.URL
	if bs == nil {
		return false
	}
	if lastBuild.Spec.Source.Blob != nil {
		return bs.URL != lastBuild.Spec.Source.Blob.URL
	}
	return true
}

func (bs *ResolvedBlobSource) RevisionChanged(lastBuild *Build) bool {
	return false
}

type ResolvedRegistrySource struct {
	Image            string   `json:"image"`
	ImagePullSecrets []string `json:"imagePullSecrets"`
}

func (rs *ResolvedRegistrySource) ImagePullSecretsVolume() corev1.Volume {
	if len(rs.ImagePullSecrets) > 0 {
		return corev1.Volume{
			Name: imagePullSecretsDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: rs.ImagePullSecrets[0],
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

func (rs *ResolvedRegistrySource) BuildEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "REGISTRY_IMAGE",
			Value: rs.Image,
		},
		homeEnv,
	}
}

func (rs *ResolvedRegistrySource) IsUnknown() bool {
	return false
}

func (rs *ResolvedRegistrySource) IsPollable() bool {
	return false
}

func (rs *ResolvedRegistrySource) ConfigChanged(lastBuild *Build) bool {
	// return rs != nil && (rs.Image != lastBuild.Spec.Source.Registry.Image || !equality.Semantic.DeepEqual(rs.ImagePullSecrets, lastBuild.Spec.Source.Registry.ImagePullSecrets))
	if rs == nil {
		return false
	}
	if lastBuild.Spec.Source.Registry != nil {
		return rs.Image != lastBuild.Spec.Source.Registry.Image || !equality.Semantic.DeepEqual(rs.ImagePullSecrets, lastBuild.Spec.Source.Registry.ImagePullSecrets)
	}
	return true
}

func (rs *ResolvedRegistrySource) RevisionChanged(lastBuild *Build) bool {
	return false
}
