package v1alpha1

import (
	"encoding/json"
	"fmt"
	"github.com/knative/pkg/kmeta"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SecretTemplateName           = "secret-volume-%s"
	SecretPathName               = "/var/build-secrets/%s"
	DOCKERSecretAnnotationPrefix = "build.pivotal.io/docker"
	GITSecretAnnotationPrefix    = "build.pivotal.io/git"
)

type BuildPodConfig struct {
	GitInitImage   string
	BuildInitImage string
	CredsInitImage string
	NopImage       string
}

func (b *Build) BuildPod(config BuildPodConfig, secrets []corev1.Secret) (*corev1.Pod, error) {
	const cacheDirName = "empty-dir"
	const layersDirName = "layers-dir"
	const platformDir = "platform-dir"

	const homeDir = "home-dir"
	const workspaceDir = "workspace-dir"

	var root int64 = 0

	buf, err := json.Marshal(b.Spec.Env)
	if err != nil {
		return nil, err
	}
	envVars := string(buf)

	homeDirVolume := corev1.VolumeMount{
		Name:      homeDir,
		MountPath: "/builder/home",
	}
	workspaceVolume := corev1.VolumeMount{
		Name:      workspaceDir,
		MountPath: "/workspace",
	}
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
	secretVolumes, secretVolumeMounts, secretArgs, err := b.secretVolumesArgs(secrets)
	if err != nil {
		return nil, err
	}
	volumes = append(volumes, secretVolumes...)

	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      b.PodName(),
			Namespace: b.Namespace(),
			Labels:    b.Labels,

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
					ImagePullPolicy: "IfNotPresent",
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "creds-init",
					Image:           config.CredsInitImage,
					Args:            secretArgs,
					ImagePullPolicy: "IfNotPresent",
					VolumeMounts:    append(secretVolumeMounts, homeDirVolume), //home volume
					Env: []corev1.EnvVar{
						{
							Name:  "HOME",
							Value: "/builder/home",
						},
					},
				},
				{
					Name:  "git-init",          //todo move from build-service?
					Image: config.GitInitImage, // image
					Args: []string{
						"-url",
						b.Spec.Source.Git.URL,
						"-revision",
						b.Spec.Source.Git.Revision,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "HOME",
							Value: "/builder/home",
						},
					},
					ImagePullPolicy: "IfNotPresent",
					WorkingDir:      "/workspace", //does this need to be in /workspace
					VolumeMounts: []corev1.VolumeMount{
						workspaceVolume,
						homeDirVolume,
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
							Value: b.Spec.Builder,
						},
						{
							Name:  "PLATFORM_ENV_VARS",
							Value: envVars,
						},
						{
							Name:  "HOME",
							Value: "/builder/home",
						},
					},
					Resources: b.Spec.Resources,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layersDir", //layers is already in buildpack built image
						},
						{
							Name:      cacheDirName,
							MountPath: "/cache",
						},
						{
							Name:      platformDir,
							MountPath: "/platform",
						},
						workspaceVolume,
						homeDirVolume,
					},
					ImagePullPolicy: "IfNotPresent",
				},
				{
					Name:      "detect",
					Image:     b.Spec.Builder,
					Resources: b.Spec.Resources,
					Command:   []string{"/lifecycle/detector"},
					Args: []string{
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name:      platformDir,
							MountPath: "/platform",
						},
						workspaceVolume,
					},
					ImagePullPolicy: "IfNotPresent",
				},
				{
					Name:      "restore",
					Image:     b.Spec.Builder,
					Resources: b.Spec.Resources,
					Command:   []string{"/lifecycle/restorer"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name:      cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "IfNotPresent",
				},
				{
					Name:      "analyze",
					Image:     b.Spec.Builder,
					Resources: b.Spec.Resources,
					Command:   []string{"/lifecycle/analyzer"},
					Args: []string{
						"-layers=/layers",
						"-helpers=false",
						"-group=/layers/group.toml",
						"-analyzed=/layers/analyzed.toml",
						b.Spec.Image,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						workspaceVolume,
						homeDirVolume,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "HOME",
							Value: "/builder/home",
						},
					},
					ImagePullPolicy: "IfNotPresent",
				},
				{
					Name:      "build",
					Image:     b.Spec.Builder,
					Resources: b.Spec.Resources,
					Command:   []string{"/lifecycle/builder"},
					Args: []string{
						"-layers=/layers",
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name:      platformDir,
							MountPath: "/platform",
						},
						workspaceVolume,
					},
					ImagePullPolicy: "IfNotPresent",
				},
				{
					Name:      "export",
					Image:     b.Spec.Builder,
					Resources: b.Spec.Resources,
					Command:   []string{"/lifecycle/exporter"},
					Args:      buildExporterArgs(b),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						workspaceVolume,
						homeDirVolume,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "HOME",
							Value: "/builder/home",
						},
					},
					ImagePullPolicy: "IfNotPresent",
				},
				{
					Name:      "cache",
					Image:     b.Spec.Builder,
					Resources: b.Spec.Resources,
					Command:   []string{"/lifecycle/cacher"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name:      cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "IfNotPresent",
				},
			},
			ServiceAccountName: b.Spec.ServiceAccount,
			Volumes:            volumes,
		},
	}, nil
}

func buildExporterArgs(build *Build) []string {
	args := []string{
		"-layers=/layers",
		"-helpers=false",
		"-app=/workspace",
		"-group=/layers/group.toml",
		"-analyzed=/layers/analyzed.toml",
		build.Spec.Image}
	args = append(args, build.Spec.AdditionalImageNames...)
	return args
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

func (b *Build) secretVolumesArgs(secrets []corev1.Secret) ([]corev1.Volume, []corev1.VolumeMount, []string, error) {
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
