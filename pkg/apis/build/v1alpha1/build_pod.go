package v1alpha1

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

const (
	buildInitBinary = "build-init"
	rebaseBinary    = "rebase"

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

type BuildPodImages struct {
	BuildInitImage  string
	CompletionImage string
	RebaseImage     string
}

type BuildPodBuilderConfig struct {
	BuilderSpec BuildBuilderSpec
	StackID     string
	RunImage    string
	Uid         int64
	Gid         int64
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

func (b *Build) BuildPod(config BuildPodImages, secrets []corev1.Secret, buildPodBuilderConfig BuildPodBuilderConfig) (*corev1.Pod, error) {
	if b.Rebasable(buildPodBuilderConfig.StackID) {
		secretVolumes, secretVolumeMounts, secretArgs, err := b.setupSecretVolumesAndArgs(secrets, dockerSecrets)
		if err != nil {
			return nil, err
		}

		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      b.PodName(),
				Namespace: b.Namespace,
				Labels: b.labels(map[string]string{
					BuildLabel: b.Name,
				}),
				OwnerReferences: []metav1.OwnerReference{
					*kmeta.NewControllerRef(b),
				},
			},
			Spec: corev1.PodSpec{
				ServiceAccountName: b.Spec.ServiceAccount,
				Volumes:            secretVolumes,
				RestartPolicy:      corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:            "completion",
						Image:           config.CompletionImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources:       b.Spec.Resources,
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:            "rebase",
						Image:           config.RebaseImage,
						Args:            rebaseArgs(rebaseBinary, buildPodBuilderConfig.RunImage, b.Spec.LastBuild.Image, b.Spec.Tags, secretArgs),
						ImagePullPolicy: corev1.PullIfNotPresent,
						WorkingDir:      "/workspace",
						VolumeMounts:    secretVolumeMounts,
					},
				},
			},
			Status: corev1.PodStatus{},
		}, nil
	}

	buf, err := json.Marshal(b.Spec.Env)
	if err != nil {
		return nil, err
	}
	envVars := string(buf)

	volumes := append(b.setupVolumes(), buildPodBuilderConfig.BuilderSpec.getBuilderSecretVolume())
	secretVolumes, secretVolumeMounts, secretArgs, err := b.setupSecretVolumesAndArgs(secrets, gitAndDockerSecrets)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, secretVolumes...)

	builderImage := buildPodBuilderConfig.BuilderSpec.Image

	workspaceVolume := corev1.VolumeMount{
		Name:      sourceVolume.Name,
		MountPath: sourceVolume.MountPath,
		SubPath:   b.Spec.Source.SubPath, // empty string is a nop
	}

	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      b.PodName(),
			Namespace: b.Namespace,
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
					Name:            "completion",
					Image:           config.CompletionImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Resources:       b.Spec.Resources,
				},
			},
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: &buildPodBuilderConfig.Gid,
			},
			InitContainers: []corev1.Container{
				{
					Name:  "prepare",
					Image: config.BuildInitImage,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &buildPodBuilderConfig.Uid,
						RunAsGroup: &buildPodBuilderConfig.Gid,
					},
					Args: buildInitArgs(buildInitBinary, secretArgs),
					Env: append(
						b.SourceEnvVars(),
						corev1.EnvVar{
							Name:  "PLATFORM_ENV_VARS",
							Value: envVars,
						},
						corev1.EnvVar{
							Name:  "IMAGE_TAG",
							Value: b.Tag(),
						},
						corev1.EnvVar{
							Name:  "RUN_IMAGE",
							Value: buildPodBuilderConfig.RunImage,
						},
					),
					ImagePullPolicy: corev1.PullIfNotPresent,
					WorkingDir:      "/workspace",
					VolumeMounts: append(
						secretVolumeMounts,
						builderPullSecretsVolume,
						imagePullSecretsVolume,
						platformVolume,
						sourceVolume,
						homeVolume,
					),
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
					Args: append([]string{
						"-layers=/layers",
						"-helpers=false",
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-analyzed=/layers/analyzed.toml",
					}, b.Spec.Tags...),
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
			},
			ServiceAccountName: b.Spec.ServiceAccount,
			Volumes:            volumes,
			ImagePullSecrets:   buildPodBuilderConfig.BuilderSpec.ImagePullSecrets,
		},
	}, nil
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

func gitAndDockerSecrets(secret corev1.Secret) bool {
	return secret.Annotations[GITSecretAnnotationPrefix] != "" || secret.Annotations[DOCKERSecretAnnotationPrefix] != ""
}

func dockerSecrets(secret corev1.Secret) bool {
	return secret.Annotations[DOCKERSecretAnnotationPrefix] != ""
}

func (b *Build) setupSecretVolumesAndArgs(secrets []corev1.Secret, filter func(secret corev1.Secret) bool) ([]corev1.Volume, []corev1.VolumeMount, []string, error) {
	var (
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		args         []string
	)
	for _, secret := range secrets {
		if !filter(secret) {
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

		if secret.Annotations[DOCKERSecretAnnotationPrefix] != "" {
			annotatedUrl := secret.Annotations[DOCKERSecretAnnotationPrefix]
			secretType := "docker"

			args = append(args, fmt.Sprintf("-basic-%s=%s=%s", secretType, secret.Name, annotatedUrl))
		} else {
			annotatedUrl := secret.Annotations[GITSecretAnnotationPrefix]
			secretType := "git"

			switch secret.Type {
			case corev1.SecretTypeBasicAuth:
				args = append(args, fmt.Sprintf("-basic-%s=%s=%s", secretType, secret.Name, annotatedUrl))
			case corev1.SecretTypeSSHAuth:
				args = append(args, fmt.Sprintf("-ssh-%s=%s=%s", secretType, secret.Name, annotatedUrl))
			}
		}
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

func combineArgs(args ...[]string) []string {
	var combined []string
	for _, a := range args {
		combined = append(combined, a...)
	}
	return combined
}

const directExecute = "--"

func rebaseArgs(rebaseBinary, runsImage, lastBuiltImage string, tags, secretArgs []string) []string {
	return combineArgs(
		[]string{directExecute, rebaseBinary},
		secretArgs,
		[]string{"--run-image", runsImage, "--last-built-image", lastBuiltImage},
		tags,
	)
}

func buildInitArgs(buildInitBinary string, secretArgs []string) []string {
	return combineArgs(
		[]string{directExecute, buildInitBinary},
		secretArgs,
	)
}
