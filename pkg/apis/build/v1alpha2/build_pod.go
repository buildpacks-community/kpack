package v1alpha2

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const (
	directExecute   = "--"
	buildInitBinary = "build-init"
	rebaseBinary    = "rebase"

	SecretTemplateName           = "secret-volume-%s"
	SecretPathName               = "/var/build-secrets/%s"
	BuildLabel                   = "kpack.io/build"
	DOCKERSecretAnnotationPrefix = "kpack.io/docker"
	GITSecretAnnotationPrefix    = "kpack.io/git"

	cacheDirName              = "cache-dir"
	layersDirName             = "layers-dir"
	platformDir               = "platform-dir"
	homeDir                   = "home-dir"
	workspaceDir              = "workspace-dir"
	imagePullSecretsDirName   = "image-pull-secrets-dir"
	builderPullSecretsDirName = "builder-pull-secrets-dir"

	serviceBindingRootEnvVar = "SERVICE_BINDING_ROOT"
)

type BuildPodImages struct {
	BuildInitImage  string
	CompletionImage string
	RebaseImage     string
}

type BuildPodBuilderConfig struct {
	StackID     string
	RunImage    string
	Uid         int64
	Gid         int64
	PlatformAPI string
}

type ServiceBinding struct {
	Name                string
	SecretRef           *corev1.LocalObjectReference
	V1Alpha1MetadataRef *corev1.LocalObjectReference
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
	projectMetadataVolume = corev1.VolumeMount{
		Name:      layersDirName,
		MountPath: "/projectMetadata",
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
	serviceBindingRootEnv = corev1.EnvVar{
		Name:  serviceBindingRootEnvVar,
		Value: filepath.Join(platformVolume.MountPath, "bindings"),
	}
)

func (b *Build) BuildPod(config BuildPodImages, secrets []corev1.Secret, bc BuildPodBuilderConfig, bindings []ServiceBinding) (*corev1.Pod, error) {
	if bc.unsupported() {
		return nil, errors.Errorf("incompatible builder platform API version: %s", bc.PlatformAPI)
	}

	if b.rebasable(bc.StackID) {
		return b.rebasePod(secrets, config, bc)
	}

	envVars, err := json.Marshal(b.Spec.Env)
	if err != nil {
		return nil, err
	}

	secretVolumes, secretVolumeMounts, secretArgs := b.setupSecretVolumesAndArgs(secrets, gitAndDockerSecrets)

	bindingVolumes, bindingVolumeMounts := b.setupBindings(bindings)

	builderImage := b.Spec.Builder.Image

	workspaceVolume := corev1.VolumeMount{
		Name:      sourceVolume.Name,
		MountPath: sourceVolume.MountPath,
		SubPath:   b.Spec.Source.SubPath, // empty string is a nop
	}

	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:      b.PodName(),
			Namespace: b.Namespace,
			Labels: combine(b.Labels, map[string]string{
				BuildLabel: b.Name,
			}),
			Annotations: b.Annotations,
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
				FSGroup: &bc.Gid,
			},
			InitContainers: steps(func(step func(corev1.Container)) {
				step(
					corev1.Container{
						Name:  "prepare",
						Image: config.BuildInitImage,
						SecurityContext: &corev1.SecurityContext{
							RunAsUser:  &bc.Uid,
							RunAsGroup: &bc.Gid,
						},
						Args: args(a(
							directExecute,
							buildInitBinary),
							secretArgs,
						),
						Env: append(
							b.Spec.Source.Source().BuildEnvVars(),
							corev1.EnvVar{
								Name:  "PLATFORM_ENV_VARS",
								Value: string(envVars),
							},
							corev1.EnvVar{
								Name:  "IMAGE_TAG",
								Value: b.Tag(),
							},
							corev1.EnvVar{
								Name:  "RUN_IMAGE",
								Value: bc.RunImage,
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
							projectMetadataVolume,
						),
					},
				)
				step(
					corev1.Container{
						Name:    "detect",
						Image:   builderImage,
						Command: []string{"/cnb/lifecycle/detector"},
						Args: []string{
							"-app=/workspace",
							"-group=/layers/group.toml",
							"-plan=/layers/plan.toml",
						},
						VolumeMounts: append([]corev1.VolumeMount{
							layersVolume,
							platformVolume,
							workspaceVolume,
						}, bindingVolumeMounts...),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env:             []corev1.EnvVar{serviceBindingRootEnv},
					},
				)
				step(
					corev1.Container{
						Name:    "analyze",
						Image:   builderImage,
						Command: []string{"/cnb/lifecycle/analyzer"},
						Args: []string{
							"-layers=/layers",
							"-group=/layers/group.toml",
							"-analyzed=/layers/analyzed.toml",
							"-cache-dir=/cache",
							func() string {
								if b.Spec.LastBuild != nil && b.Spec.LastBuild.Image != "" {
									return b.Spec.LastBuild.Image
								}
								return b.Tag()
							}(),
						},
						VolumeMounts: []corev1.VolumeMount{
							layersVolume,
							workspaceVolume,
							homeVolume,
							cacheVolume,
						},
						Env: []corev1.EnvVar{
							homeEnv,
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				)
				step(
					corev1.Container{
						Name:    "restore",
						Image:   builderImage,
						Command: []string{"/cnb/lifecycle/restorer"},
						Args: []string{
							"-group=/layers/group.toml",
							"-layers=/layers",
							"-cache-dir=/cache",
						},
						VolumeMounts: []corev1.VolumeMount{
							layersVolume,
							cacheVolume,
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
				)
				step(
					corev1.Container{
						Name:    "build",
						Image:   builderImage,
						Command: []string{"/cnb/lifecycle/builder"},
						Args: []string{
							"-layers=/layers",
							"-app=/workspace",
							"-group=/layers/group.toml",
							"-plan=/layers/plan.toml",
						},
						VolumeMounts: append([]corev1.VolumeMount{
							layersVolume,
							platformVolume,
							workspaceVolume,
						}, bindingVolumeMounts...),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env:             []corev1.EnvVar{serviceBindingRootEnv},
					},
				)
				if bc.legacy() {
					step(
						corev1.Container{
							Name:    "export",
							Image:   builderImage,
							Command: []string{"/cnb/lifecycle/exporter"},
							Args: append([]string{
								"-layers=/layers",
								"-app=/workspace",
								"-group=/layers/group.toml",
								"-analyzed=/layers/analyzed.toml",
								"-cache-dir=/cache",
							}, b.Spec.Tags...),
							VolumeMounts: []corev1.VolumeMount{
								layersVolume,
								workspaceVolume,
								homeVolume,
								cacheVolume,
							},
							Env: []corev1.EnvVar{
								homeEnv,
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					)
				} else {
					step(
						corev1.Container{
							Name:    "export",
							Image:   builderImage,
							Command: []string{"/cnb/lifecycle/exporter"},
							Args: append([]string{
								"-layers=/layers",
								"-app=/workspace",
								"-group=/layers/group.toml",
								"-analyzed=/layers/analyzed.toml",
								"-cache-dir=/cache",
								"-project-metadata=/layers/project-metadata.toml",
							}, b.Spec.Tags...),
							VolumeMounts: []corev1.VolumeMount{
								layersVolume,
								workspaceVolume,
								homeVolume,
								cacheVolume,
							},
							Env: []corev1.EnvVar{
								homeEnv,
							},
							ImagePullPolicy: corev1.PullIfNotPresent,
						},
					)
				}
			}),
			ServiceAccountName: b.Spec.ServiceAccount,
			Volumes: append(append(
				secretVolumes,
				corev1.Volume{
					Name:         cacheDirName,
					VolumeSource: b.cacheVolume(),
				},
				corev1.Volume{
					Name: layersDirName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				corev1.Volume{
					Name: homeDir,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				corev1.Volume{
					Name: workspaceDir,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				corev1.Volume{
					Name: platformDir,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				b.Spec.Source.Source().ImagePullSecretsVolume(),
				builderSecretVolume(b.Spec.Builder),
			), bindingVolumes...),
			ImagePullSecrets: b.Spec.Builder.ImagePullSecrets,
		},
	}, nil
}

func (b *Build) rebasePod(secrets []corev1.Secret, config BuildPodImages, buildPodBuilderConfig BuildPodBuilderConfig) (*corev1.Pod, error) {
	secretVolumes, secretVolumeMounts, secretArgs := b.setupSecretVolumesAndArgs(secrets, dockerSecrets)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.PodName(),
			Namespace: b.Namespace,
			Labels: combine(b.Labels, map[string]string{
				BuildLabel: b.Name,
			}),
			Annotations: b.Annotations,
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
					Name:  "rebase",
					Image: config.RebaseImage,
					Args: args(a(
						directExecute,
						rebaseBinary,
						"--run-image",
						buildPodBuilderConfig.RunImage,
						"--last-built-image",
						b.Spec.LastBuild.Image),
						secretArgs,
						b.Spec.Tags,
					),
					ImagePullPolicy: corev1.PullIfNotPresent,
					WorkingDir:      "/workspace",
					VolumeMounts:    secretVolumeMounts,
				},
			},
		},
		Status: corev1.PodStatus{},
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
	return secret.Annotations[GITSecretAnnotationPrefix] != "" || dockerSecrets(secret)
}

func dockerSecrets(secret corev1.Secret) bool {
	return secret.Annotations[DOCKERSecretAnnotationPrefix] != "" || secret.Type == corev1.SecretTypeDockercfg || secret.Type == corev1.SecretTypeDockerConfigJson
}

func (b *Build) setupSecretVolumesAndArgs(secrets []corev1.Secret, filter func(secret corev1.Secret) bool) ([]corev1.Volume, []corev1.VolumeMount, []string) {
	var (
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		args         []string
	)
	for _, secret := range secrets {
		switch {
		case !filter(secret):
			continue
		case secret.Type == corev1.SecretTypeBasicAuth && secret.Annotations[DOCKERSecretAnnotationPrefix] != "":
			args = append(args,
				fmt.Sprintf("-basic-%s=%s=%s", "docker", secret.Name, secret.Annotations[DOCKERSecretAnnotationPrefix]))
		case secret.Type == corev1.SecretTypeDockerConfigJson:
			args = append(args, fmt.Sprintf("-dockerconfig=%s", secret.Name))
		case secret.Type == corev1.SecretTypeDockercfg:
			args = append(args, fmt.Sprintf("-dockercfg=%s", secret.Name))
		case secret.Type == corev1.SecretTypeBasicAuth && secret.Annotations[GITSecretAnnotationPrefix] != "":
			annotatedUrl := secret.Annotations[GITSecretAnnotationPrefix]
			args = append(args, fmt.Sprintf("-basic-%s=%s=%s", "git", secret.Name, annotatedUrl))
		case secret.Type == corev1.SecretTypeSSHAuth:
			annotatedUrl := secret.Annotations[GITSecretAnnotationPrefix]
			args = append(args, fmt.Sprintf("-ssh-%s=%s=%s", "git", secret.Name, annotatedUrl))
		default:
			//ignoring secret
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
	}

	return volumes, volumeMounts, args
}

func (b *Build) setupBindings(bindings []ServiceBinding) ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	for _, s := range bindings {
		if s.SecretRef != nil {
			secretVolume := fmt.Sprintf("service-binding-secret-%s", s.Name)
			volumes = append(volumes,
				corev1.Volume{
					Name: secretVolume,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: s.SecretRef.Name,
						},
					},
				},
			)
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      secretVolume,
					MountPath: filepath.Join(platformVolume.MountPath, "bindings", s.Name),
					ReadOnly:  true,
				},
			)
		}
		// If this is a v1alpha1 binding
		if s.V1Alpha1MetadataRef != nil {
			metadataVolume := fmt.Sprintf("service-binding-metadata-%s", s.Name)
			volumes = append(volumes,
				corev1.Volume{
					Name: metadataVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: *s.V1Alpha1MetadataRef,
						},
					},
				},
			)
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      metadataVolume,
					MountPath: filepath.Join(platformVolume.MountPath, "bindings", s.Name, "metadata"),
					ReadOnly:  true,
				},
			)
		}
	}

	return volumes, volumeMounts
}

func (bc *BuildPodBuilderConfig) legacy() bool {
	return bc.PlatformAPI == "0.2"
}

func (bc *BuildPodBuilderConfig) unsupported() bool {
	return bc.PlatformAPI != "0.2" && bc.PlatformAPI != "0.3"
}

func builderSecretVolume(bbs v1alpha1.BuildBuilderSpec) corev1.Volume {
	if len(bbs.ImagePullSecrets) > 0 {
		return corev1.Volume{
			Name: builderPullSecretsDirName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: bbs.ImagePullSecrets[0].Name,
				},
			},
		}
	} else {
		return corev1.Volume{
			Name: builderPullSecretsDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}
}

func args(args ...[]string) []string {
	var combined []string
	for _, a := range args {
		combined = append(combined, a...)
	}
	return combined
}

func a(args ...string) []string {
	return args
}

func steps(f func(step func(corev1.Container))) []corev1.Container {
	containers := make([]corev1.Container, 0, 7)
	f(func(container corev1.Container) {
		containers = append(containers, container)
	})
	return containers
}
