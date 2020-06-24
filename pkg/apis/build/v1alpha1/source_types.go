package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

// +k8s:openapi-gen=true
type SourceConfig struct {
	Git      *Git      `json:"git,omitempty"`
	Blob     *Blob     `json:"blob,omitempty"`
	Registry *Registry `json:"registry,omitempty"`
	SubPath  string    `json:"subPath,omitempty"`
	S3       *S3       `json:"s3,omitempty"`
}

func (sc *SourceConfig) Source() Source {
	if sc.Git != nil {
		return sc.Git
	} else if sc.Blob != nil {
		return sc.Blob
	} else if sc.Registry != nil {
		return sc.Registry
	} else if sc.S3 != nil {
		return sc.S3
	}
	return nil
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

// +k8s:openapi-gen=true
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
	}
}

// +k8s:openapi-gen=true
type Registry struct {
	Image string `json:"image"`
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`
}

func (r *Registry) ImagePullSecretsVolume() corev1.Volume {
	if len(r.ImagePullSecrets) > 0 {
		return corev1.Volume{
			Name: imagePullSecretsDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: r.ImagePullSecrets[0].Name,
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
	}
}

// +k8s:openapi-gen=true
type S3 struct {
	URL            string `json:"url"`
	AccessKey      string `json:"accesskey"`
	SecretKey      string `json:"secretkey"`
	Bucket         string `json:"bucket"`
	File           string `json:"file"`
	Region         string `json:"region"`
	ForcePathStyle string `json:"forcePathStyle"`
}

func (s *S3) ImagePullSecretsVolume() corev1.Volume {
	return corev1.Volume{
		Name: imagePullSecretsDirName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func (s *S3) BuildEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name:  "S3_URL",
			Value: s.URL,
		},
		{
			Name:  "S3_ACCESS_KEY",
			Value: s.AccessKey,
		},
		{
			Name:  "S3_SECRET_KEY",
			Value: s.SecretKey,
		},
		{
			Name:  "S3_BUCKET",
			Value: s.Bucket,
		},
		{
			Name:  "S3_FILE",
			Value: s.File,
		},
		{
			Name:  "S3_FORCE_PATH_STYLE",
			Value: s.ForcePathStyle,
		},
		{
			Name:  "S3_REGION",
			Value: s.Region,
		},
	}
}

// +k8s:openapi-gen=true
type ResolvedSourceConfig struct {
	Git      *ResolvedGitSource      `json:"git,omitempty"`
	Blob     *ResolvedBlobSource     `json:"blob,omitempty"`
	Registry *ResolvedRegistrySource `json:"registry,omitempty"`
	S3       *ResolvedS3Source       `json:"s3,omitempty"`
}

func (sc ResolvedSourceConfig) ResolvedSource() ResolvedSource {
	if sc.Git != nil {
		return sc.Git
	} else if sc.Blob != nil {
		return sc.Blob
	} else if sc.Registry != nil {
		return sc.Registry
	} else if sc.S3 != nil {
		return sc.S3
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

type ResolvedS3Source struct {
	URL            string `json:"string"`
	AccessKey      string `json:"accessKey"`
	SecretKey      string `json:"secretKey"`
	Bucket         string `json:"bucket"`
	File           string `json:"file"`
	SubPath        string `json:"subPath,omitempty"`
	Region         string `json:"region,omitempty"`
	ForcePathStyle string `json:"forcePathStyle,omitempty"`
}

func (rs *ResolvedS3Source) SourceConfig() SourceConfig {
	return SourceConfig{
		S3: &S3{
			URL:            rs.URL,
			AccessKey:      rs.AccessKey,
			SecretKey:      rs.SecretKey,
			Bucket:         rs.Bucket,
			File:           rs.File,
			ForcePathStyle: rs.ForcePathStyle,
			Region:         rs.Region,
		},
		SubPath: rs.SubPath,
	}
}

func (rs *ResolvedS3Source) IsUnknown() bool {
	return false
}

func (rs *ResolvedS3Source) IsPollable() bool {
	return false
}

func (rs *ResolvedS3Source) ConfigChanged(lastBuild *Build) bool {
	if lastBuild.Spec.Source.S3 == nil {
		return true
	}

	return rs.URL != lastBuild.Spec.Source.S3.URL ||
		rs.AccessKey != lastBuild.Spec.Source.S3.AccessKey ||
		rs.SecretKey != lastBuild.Spec.Source.S3.SecretKey ||
		rs.Bucket != lastBuild.Spec.Source.S3.Bucket ||
		rs.File != lastBuild.Spec.Source.S3.File ||
		rs.SubPath != lastBuild.Spec.Source.SubPath ||
		rs.ForcePathStyle != lastBuild.Spec.Source.S3.ForcePathStyle ||
		rs.Region != lastBuild.Spec.Source.S3.Region
}

func (rs *ResolvedS3Source) RevisionChanged(lastBuild *Build) bool {
	return false
}
