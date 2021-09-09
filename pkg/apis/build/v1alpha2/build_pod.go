package v1alpha2

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
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

	notaryDirName = "notary-dir"
	reportDirName = "report-dir"

	networkWaitLauncherDir = "network-wait-launcher-dir"

	buildChangesEnvVar = "BUILD_CHANGES"
	platformAPIEnvVar  = "CNB_PLATFORM_API"
)

type BuildPodImages struct {
	BuildInitImage         string
	CompletionImage        string
	RebaseImage            string
	BuildInitWindowsImage  string
	CompletionWindowsImage string
}

func (bpi *BuildPodImages) buildInit(os string) string {
	switch os {
	case "windows":
		return bpi.BuildInitWindowsImage
	default:
		return bpi.BuildInitImage
	}
}

func (bpi *BuildPodImages) completion(os string) string {
	switch os {
	case "windows":
		return bpi.CompletionWindowsImage
	default:
		return bpi.CompletionImage
	}
}

type BuildPodBuilderConfig struct {
	StackID      string
	RunImage     string
	Uid          int64
	Gid          int64
	PlatformAPIs []string
	OS           string
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
	notaryV1Volume = corev1.VolumeMount{
		Name:      notaryDirName,
		MountPath: "/var/notary/v1",
		ReadOnly:  true,
	}
	reportVolume = corev1.VolumeMount{
		Name:      reportDirName,
		MountPath: "/var/report",
		ReadOnly:  false,
	}
	networkWaitLauncherVolume = corev1.VolumeMount{
		Name:      networkWaitLauncherDir,
		MountPath: "/networkWait",
		ReadOnly:  false,
	}
)

type stepModifier func(corev1.Container) corev1.Container

func (b *Build) BuildPod(images BuildPodImages, secrets []corev1.Secret, taints []corev1.Taint, config BuildPodBuilderConfig) (*corev1.Pod, error) {
	platformAPI, err := config.highestSupportedPlatformAPI(b)
	if err != nil {
		return nil, err
	}

	if b.rebasable(config.StackID) {
		return b.rebasePod(secrets, images, config)
	}

	ref, err := name.ParseReference(b.Tag())
	if err != nil {
		return nil, err
	}
	dnsProbeHost := ref.Context().RegistryStr()

	envVars, err := json.Marshal(b.Spec.Env)
	if err != nil {
		return nil, err
	}

	secretVolumes, secretVolumeMounts, secretArgs := b.setupSecretVolumesAndArgs(secrets, gitAndDockerSecrets)

	bindingVolumes, bindingVolumeMounts := b.setupBindings()

	builderImage := b.Spec.Builder.Image

	workspaceVolume := corev1.VolumeMount{
		Name:      sourceVolume.Name,
		MountPath: sourceVolume.MountPath,
		SubPath:   b.Spec.Source.SubPath, // empty string is a nop
	}

	var genericCacheArgs []string
	var exporterCacheArgs []string
	var cacheVolumes []corev1.VolumeMount

	if (!b.Spec.NeedVolumeCache() && !b.Spec.NeedRegistryCache()) || config.OS == "windows" {
		genericCacheArgs = nil
	} else if b.Spec.NeedRegistryCache() {
		useCacheFromLastBuild := (b.Spec.LastBuild != nil && b.Spec.LastBuild.Cache.Image != "")
		if useCacheFromLastBuild {
			genericCacheArgs = []string{fmt.Sprintf("-cache-image=%s", b.Spec.LastBuild.Cache.Image)}
		}
		exporterCacheArgs = []string{fmt.Sprintf("-cache-image=%s", b.Spec.Cache.Registry.Tag)}
	} else {
		genericCacheArgs = []string{"-cache-dir=/cache"}
		cacheVolumes = []corev1.VolumeMount{cacheVolume}
		exporterCacheArgs = genericCacheArgs
	}

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
			// If the build fails, don't restart it.
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: steps(func(step func(corev1.Container, ...stepModifier)) {
				if b.NotaryV1Config() == nil {
					step(corev1.Container{
						Name:            "completion",
						Image:           images.completion(config.OS),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources:       b.Spec.Resources,
					})
					return
				}

				_, notarySecretVolumeMounts, notarySecretArgs := b.setupSecretVolumesAndArgs(secrets, dockerSecrets)
				step(corev1.Container{
					Name:    "completion",
					Image:   images.completion(config.OS),
					Command: []string{"/cnb/process/web"},
					Args: append(
						[]string{
							"-notary-v1-url=" + b.NotaryV1Config().URL,
						},
						notarySecretArgs...,
					),
					Resources: b.Spec.Resources,
					VolumeMounts: append(
						notarySecretVolumeMounts,
						notaryV1Volume,
						reportVolume,
					),
					ImagePullPolicy: corev1.PullIfNotPresent,
				}, ifWindows(config.OS, addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))...)
			}),
			SecurityContext: podSecurityContext(config),
			InitContainers: steps(func(step func(corev1.Container, ...stepModifier)) {
				step(
					corev1.Container{
						Name:  "prepare",
						Image: images.buildInit(config.OS),
						Args:  secretArgs,
						Env: append(
							b.Spec.Source.Source().BuildEnvVars(),
							corev1.EnvVar{
								Name:  "SOURCE_SUB_PATH",
								Value: b.Spec.Source.SubPath,
							},
							corev1.EnvVar{
								Name:  "PROJECT_DESCRIPTOR_PATH",
								Value: b.Spec.ProjectDescriptorPath,
							},
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
								Value: config.RunImage,
							},
							corev1.EnvVar{
								Name:  "DNS_PROBE_HOSTNAME",
								Value: dnsProbeHost,
							},
							corev1.EnvVar{
								Name:  buildChangesEnvVar,
								Value: b.BuildChanges(),
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
					ifWindows(config.OS, addNetworkWaitLauncherVolume())...,
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
						Env: []corev1.EnvVar{
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
						},
					},
					ifWindows(config.OS, addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))...,
				)
				step(
					corev1.Container{
						Name:    "analyze",
						Image:   builderImage,
						Command: []string{"/cnb/lifecycle/analyzer"},
						Args: args([]string{
							"-layers=/layers",
							"-group=/layers/group.toml",
							"-analyzed=/layers/analyzed.toml"},
							genericCacheArgs,
							func() []string {
								if b.Spec.LastBuild != nil && b.Spec.LastBuild.Image != "" {
									return []string{b.Spec.LastBuild.Image}
								}
								return []string{b.Tag()}
							}(),
						),
						VolumeMounts: append([]corev1.VolumeMount{
							layersVolume,
							workspaceVolume,
							homeVolume,
						}, cacheVolumes...),
						Env: []corev1.EnvVar{
							homeEnv,
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
					ifWindows(config.OS,
						addNetworkWaitLauncherVolume(),
						useNetworkWaitLauncher(dnsProbeHost),
						userprofileHomeEnv(),
					)...,
				)
				step(
					corev1.Container{
						Name:    "restore",
						Image:   builderImage,
						Command: []string{"/cnb/lifecycle/restorer"},
						Args: args([]string{
							"-group=/layers/group.toml",
							"-layers=/layers",
						}, genericCacheArgs),
						VolumeMounts: append([]corev1.VolumeMount{
							layersVolume,
							homeVolume,
						}, cacheVolumes...),
						Env: []corev1.EnvVar{
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
					ifWindows(config.OS, addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))...,
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
						Env: []corev1.EnvVar{
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
						},
					},
					ifWindows(config.OS, addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))...,
				)
				step(
					corev1.Container{
						Name:    "export",
						Image:   builderImage,
						Command: []string{"/cnb/lifecycle/exporter"},
						Args: args([]string{
							"-layers=/layers",
							"-app=/workspace",
							"-group=/layers/group.toml",
							"-analyzed=/layers/analyzed.toml",
							"-project-metadata=/layers/project-metadata.toml"},
							exporterCacheArgs,
							func() []string {
								if b.DefaultProcess() == "" {
									if platformAPI.Equal(lowestSupportedPlatformVersion) || platformAPI.Equal(highestSupportedPlatformVersion) {
										return nil
									} else {
										return []string{fmt.Sprintf("-process-type=web")}
									}
								}
								return []string{fmt.Sprintf("-process-type=%s", b.DefaultProcess())}
							}(),
							func() []string {
								if platformAPI.Equal(lowestSupportedPlatformVersion) {
									return nil
								}
								return []string{"-report=/var/report/report.toml"}

							}(),
							b.Spec.Tags),
						VolumeMounts: append([]corev1.VolumeMount{
							layersVolume,
							workspaceVolume,
							homeVolume,
							reportVolume,
						}, cacheVolumes...),
						Env: []corev1.EnvVar{
							homeEnv,
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
					ifWindows(config.OS,
						addNetworkWaitLauncherVolume(),
						useNetworkWaitLauncher(dnsProbeHost),
						userprofileHomeEnv(),
					)...,
				)
			}),
			ServiceAccountName: b.Spec.ServiceAccount,
			NodeSelector: map[string]string{
				"kubernetes.io/os": config.OS,
			},
			Tolerations: tolerations(taints),
			Volumes: append(append(
				append(secretVolumes, b.cacheVolume(config.OS)...),
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
				corev1.Volume{
					Name: reportDirName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				corev1.Volume{
					Name: networkWaitLauncherDir,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				b.Spec.Source.Source().ImagePullSecretsVolume(),
				builderSecretVolume(b.Spec.Builder),
				b.notarySecretVolume(),
			), bindingVolumes...),
			ImagePullSecrets: b.Spec.Builder.ImagePullSecrets,
		},
	}, nil
}

func podSecurityContext(config BuildPodBuilderConfig) *corev1.PodSecurityContext {
	if config.OS == "windows" {
		return nil
	}

	return &corev1.PodSecurityContext{
		FSGroup:    &config.Gid,
		RunAsUser:  &config.Uid,
		RunAsGroup: &config.Gid,
	}
}

func ifWindows(os string, modifiers ...stepModifier) []stepModifier {
	if os == "windows" {
		return modifiers
	}

	return []stepModifier{noOpModifer}
}

func useNetworkWaitLauncher(dnsProbeHost string) stepModifier {
	return func(container corev1.Container) corev1.Container {
		startCommand := container.Command
		container.Args = args([]string{dnsProbeHost, "--"}, startCommand, container.Args)

		container.Command = []string{"/networkWait/network-wait-launcher"}
		return container
	}
}

func addNetworkWaitLauncherVolume() stepModifier {
	return func(container corev1.Container) corev1.Container {
		container.VolumeMounts = append(container.VolumeMounts, networkWaitLauncherVolume)
		return container
	}
}

func userprofileHomeEnv() stepModifier {
	return func(container corev1.Container) corev1.Container {
		for i, env := range container.Env {
			if env.Name == "HOME" {
				container.Env[i].Name = "USERPROFILE"
			}
		}

		return container
	}
}

func noOpModifer(container corev1.Container) corev1.Container {
	return container
}

func (b *Build) notarySecretVolume() corev1.Volume {
	config := b.NotaryV1Config()
	if config == nil {
		return corev1.Volume{
			Name: notaryDirName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}

	}

	return corev1.Volume{
		Name: notaryDirName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: config.SecretRef.Name,
			},
		},
	}
}

func (b *Build) rebasePod(secrets []corev1.Secret, images BuildPodImages, config BuildPodBuilderConfig) (*corev1.Pod, error) {
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
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
			},
			Volumes: append(
				secretVolumes,
				corev1.Volume{
					Name: reportDirName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				b.notarySecretVolume(),
			),
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: steps(func(step func(corev1.Container, ...stepModifier)) {
				notaryConfig := b.NotaryV1Config()

				if notaryConfig == nil {
					step(corev1.Container{
						Name:            "completion",
						Image:           images.CompletionImage,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Resources:       b.Spec.Resources,
					})
				} else {
					step(corev1.Container{
						Name:  "completion",
						Image: images.CompletionImage,
						Args: append(
							[]string{
								"-notary-v1-url=" + notaryConfig.URL,
							},
							secretArgs...,
						),
						Resources: b.Spec.Resources,
						VolumeMounts: append(
							secretVolumeMounts,
							notaryV1Volume,
							reportVolume,
						),
						ImagePullPolicy: corev1.PullIfNotPresent,
					})
				}
			}),
			InitContainers: []corev1.Container{
				{
					Name:  "rebase",
					Image: images.RebaseImage,
					Args: args(a(
						"--run-image",
						config.RunImage,
						"--last-built-image",
						b.Spec.LastBuild.Image,
						"--report",
						"/var/report/report.toml",
					),
						secretArgs,
						b.Spec.Tags,
					),
					Env: []corev1.EnvVar{
						{
							Name:  buildChangesEnvVar,
							Value: b.BuildChanges(),
						},
					},
					ImagePullPolicy: corev1.PullIfNotPresent,
					WorkingDir:      "/workspace",
					VolumeMounts:    append(secretVolumeMounts, reportVolume),
				},
			},
		},
		Status: corev1.PodStatus{},
	}, nil
}

func (b *Build) cacheVolume(os string) []corev1.Volume {
	if !b.Spec.NeedVolumeCache() || os == "windows" {
		return []corev1.Volume{}
	}

	return []corev1.Volume{{
		Name: cacheDirName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: b.Spec.Cache.Volume.ClaimName},
		},
	}}
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

func (b *Build) setupBindings() ([]corev1.Volume, []corev1.VolumeMount) {
	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}
	for _, binding := range b.Spec.Bindings {
		metadataVolume := fmt.Sprintf("binding-metadata-%s", binding.Name)
		volumes = append(volumes,
			corev1.Volume{
				Name: metadataVolume,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: binding.MetadataRef.Name,
						},
					},
				},
			},
		)
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      metadataVolume,
				MountPath: fmt.Sprintf("%s/bindings/%s/metadata", platformVolume.MountPath, binding.Name),
				ReadOnly:  true,
			},
		)
		if binding.SecretRef != nil {
			secretVolume := fmt.Sprintf("binding-secret-%s", binding.Name)
			volumes = append(volumes,
				corev1.Volume{
					Name: secretVolume,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: binding.SecretRef.Name,
						},
					},
				},
			)
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      secretVolume,
					MountPath: fmt.Sprintf("%s/bindings/%s/secret", platformVolume.MountPath, binding.Name),
					ReadOnly:  true,
				},
			)
		}
	}

	return volumes, volumeMounts
}

var (
	highestSupportedPlatformVersion = semver.MustParse("0.6")
	lowestSupportedPlatformVersion  = semver.MustParse("0.3")

	supportedPlatformAPIVersionsWithWindowsAndReportToml = []*semver.Version{semver.MustParse("0.6"), semver.MustParse("0.5"), semver.MustParse("0.4")}
	supportedPlatformAPIVersions                         = append(supportedPlatformAPIVersionsWithWindowsAndReportToml, semver.MustParse("0.3"))
)

func (bc *BuildPodBuilderConfig) highestSupportedPlatformAPI(b *Build) (*semver.Version, error) {
	for _, supportedVersion := range func() []*semver.Version {
		if b.NotaryV1Config() != nil || bc.OS == "windows" {
			return supportedPlatformAPIVersionsWithWindowsAndReportToml
		}
		return supportedPlatformAPIVersions
	}() {
		for _, v := range bc.PlatformAPIs {
			version, err := semver.NewVersion(v)
			if err != nil {
				return nil, errors.Wrapf(err, "unexpected platform version %s", v)
			}

			if supportedVersion.Equal(version) {
				return version, nil
			}
		}
	}

	return nil, errors.Errorf("unsupported builder platform API versions: %s", strings.Join(bc.PlatformAPIs, ","))
}

func tolerations(taints []corev1.Taint) []corev1.Toleration {
	t := make([]corev1.Toleration, 0, len(taints))

	for _, taint := range taints {
		t = append(t, corev1.Toleration{
			Key:      taint.Key,
			Operator: corev1.TolerationOpEqual,
			Value:    taint.Value,
			Effect:   taint.Effect,
		})
	}

	return t
}

func builderSecretVolume(bbs corev1alpha1.BuildBuilderSpec) corev1.Volume {
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

func steps(f func(step func(corev1.Container, ...stepModifier))) []corev1.Container {
	containers := make([]corev1.Container, 0, 7)

	f(func(container corev1.Container, modifiers ...stepModifier) {
		for _, m := range modifiers {
			container = m(container)
		}
		containers = append(containers, container)
	})
	return containers
}
