package v1alpha1

import (
	"encoding/json"
	"fmt"

	"github.com/knative/pkg/kmeta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SecretTemplateName           = "secret-volume-%s"
	SecretPathName               = "/var/build-secrets/%s"
	BuildLabel                   = "build.pivotal.io/build"
	DOCKERSecretAnnotationPrefix = "build.pivotal.io/docker"
	GITSecretAnnotationPrefix    = "build.pivotal.io/git"

	cacheDirName              = "cache-dir"
	layersDirName             = "layers-dir"
	platformDir               = "platform-dir"
	homeDir                   = "home-dir"
	workspaceDir              = "workspace-dir"
	imagePullSecretsDirName   = "image-pull-secrets-dir"
	builderPullSecretsDirName = "builder-pull-secrets-dir"
)

type BuildPodConfig struct {
	SourceInitImage string
	BuildInitImage  string
	CredsInitImage  string
	NopImage        string
}

var (
	sourceVolume = corev1.VolumeMount{
		Name:      workspaceDir,
		MountPath: "/workspace",
	}
	homeVolume = corev1.VolumeMount{
		Name:      homeDir,
		MountPath: "/builder/home",
	}
	platformVolume = corev1.VolumeMount{
		Name:      platformDir,
		MountPath: "/platform",
	}
	cacheVolume = corev1.VolumeMount{
		Name:      cacheDirName,
		MountPath: "/cache",
	}
	layersVolume = corev1.VolumeMount{
		Name:      layersDirName,
		MountPath: "/layers",
	}
	homeEnv = corev1.EnvVar{
		Name:  "HOME",
		Value: "/builder/home",
	}
	imagePullSecretsVolume = corev1.VolumeMount{
		Name:      imagePullSecretsDirName,
		MountPath: "/imagePullSecrets",
		ReadOnly:  true,
	}
	builderPullSecretsVolume = corev1.VolumeMount{
		Name:      builderPullSecretsDirName,
		MountPath: "/builderPullSecrets",
		ReadOnly:  true,
	}
)

func (b *Build) BuildPod(config BuildPodConfig, secrets []corev1.Secret, builder BuilderImage) (*corev1.Pod, error) {
	var root int64 = 0

	buf, err := json.Marshal(b.Spec.Env)
	if err != nil {
		return nil, err
	}
	envVars := string(buf)

	volumes := append(b.setupVolumes(), builder.getBuilderSecretVolume())
	secretVolumes, secretVolumeMounts, secretArgs, err := b.setupSecretVolumesAndArgs(secrets)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, secretVolumes...)

	builderImage := builder.Image

	workspaceVolume := corev1.VolumeMount{
		Name:      sourceVolume.Name,
		MountPath: sourceVolume.MountPath,
		SubPath:   b.Spec.Source.SubPath, // empty string is a nop
	}

	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      b.PodName(),
			Namespace: b.Namespace(),
			Labels: b.labels(map[string]string{
				BuildLabel: b.Name,
			}),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
		},
		Spec: corev1.PodSpec{
			// If the build fails, don't restart it.
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "nop",
					Image:           config.NopImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Resources:       b.Spec.Resources,
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "creds-init",
					Image:           config.CredsInitImage,
					Args:            secretArgs,
					ImagePullPolicy: corev1.PullIfNotPresent,
					VolumeMounts:    append(secretVolumeMounts, homeVolume),
					Env: []corev1.EnvVar{
						{
							Name:  "HOME",
							Value: "/builder/home",
						},
					},
				},
				{
					Name:  "source-init",
					Image: config.SourceInitImage,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &root,
						RunAsGroup: &root,
					},
					Env:             b.BuildEnvVars(),
					ImagePullPolicy: corev1.PullIfNotPresent,
					WorkingDir:      "/workspace",
					VolumeMounts: []corev1.VolumeMount{
						imagePullSecretsVolume,
						sourceVolume,
						homeVolume,
					},
				},
				{
					Name:  "prepare",
					Image: config.BuildInitImage,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &root,
						RunAsGroup: &root,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "BUILDER",
							Value: builderImage,
						},
						{
							Name:  "PLATFORM_ENV_VARS",
							Value: envVars,
						},
						{
							Name:  "IMAGE_TAG",
							Value: b.Tag(),
						},
						homeEnv,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layersDir",
						},
						cacheVolume,
						platformVolume,
						workspaceVolume,
						homeVolume,
						builderPullSecretsVolume,
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:    "detect",
					Image:   builderImage,
					Command: []string{"/lifecycle/detector"},
					Args: []string{
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						layersVolume,
						platformVolume,
						workspaceVolume,
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:    "restore",
					Image:   builderImage,
					Command: []string{"/lifecycle/restorer"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						layersVolume,
						cacheVolume,
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:    "analyze",
					Image:   builderImage,
					Command: []string{"/lifecycle/analyzer"},
					Args: []string{
						"-layers=/layers",
						"-helpers=false",
						"-group=/layers/group.toml",
						"-analyzed=/layers/analyzed.toml",
						b.Tag(),
					},
					VolumeMounts: []corev1.VolumeMount{
						layersVolume,
						workspaceVolume,
						homeVolume,
					},
					Env: []corev1.EnvVar{
						homeEnv,
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:    "build",
					Image:   builderImage,
					Command: []string{"/lifecycle/builder"},
					Args: []string{
						"-layers=/layers",
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						layersVolume,
						platformVolume,
						workspaceVolume,
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:    "export",
					Image:   builderImage,
					Command: []string{"/lifecycle/exporter"},
					Args:    buildExporterArgs(b),
					VolumeMounts: []corev1.VolumeMount{
						layersVolume,
						workspaceVolume,
						homeVolume,
					},
					Env: []corev1.EnvVar{
						homeEnv,
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:    "cache",
					Image:   builderImage,
					Command: []string{"/lifecycle/cacher"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						layersVolume,
						cacheVolume,
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:            "success",
					Image:           config.NopImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
			ServiceAccountName: b.Spec.ServiceAccount,
			Volumes:            volumes,
			ImagePullSecrets:   builder.ImagePullSecrets,
		},
	}, nil
}

func buildExporterArgs(build *Build) []string {
	return append([]string{
		"-layers=/layers",
		"-helpers=false",
		"-app=/workspace",
		"-group=/layers/group.toml",
		"-analyzed=/layers/analyzed.toml",
	}, build.Spec.Tags...)
}

func (b *Build) cacheVolume() corev1.VolumeSource {
	if b.Spec.CacheName != "" {
		return corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: b.Spec.CacheName},
		}
	} else {
		return corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	}
}

func isBuildServiceSecret(secret corev1.Secret) bool {
	return secret.Annotations[GITSecretAnnotationPrefix] != "" || secret.Annotations[DOCKERSecretAnnotationPrefix] != ""
}

func (b *Build) setupSecretVolumesAndArgs(secrets []corev1.Secret) ([]corev1.Volume, []corev1.VolumeMount, []string, error) {
	var (
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		args         []string
	)
	for _, secret := range secrets {
		if !isBuildServiceSecret(secret) {
			continue
		}
		volumeName := fmt.Sprintf(SecretTemplateName, secret.Name)

		volumes = append(volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secret.Name,
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf(SecretPathName, secret.Name),
		})

		annotatedUrl := secret.Annotations[DOCKERSecretAnnotationPrefix]
		secretType := "docker"
		if secret.Annotations[GITSecretAnnotationPrefix] != "" {
			annotatedUrl = secret.Annotations[GITSecretAnnotationPrefix]
			secretType = "git"
		}

		args = append(args, fmt.Sprintf("-basic-%s=%s=%s", secretType, secret.Name, annotatedUrl))
	}

	return volumes, volumeMounts, args, nil
}

func (b *Build) setupVolumes() []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name:         cacheDirName,
			VolumeSource: b.cacheVolume(),
		},
		{
			Name: layersDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: homeDir,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: workspaceDir,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: platformDir,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	return append(volumes, b.ImagePullSecretsVolume())
}

func (b *Build) labels(additionalLabels map[string]string) map[string]string {
	labels := make(map[string]string, len(additionalLabels)+len(b.Labels))

	for k, v := range b.Labels {
		labels[k] = v
	}
	for k, v := range additionalLabels {
		labels[k] = v
	}
	return labels
}
