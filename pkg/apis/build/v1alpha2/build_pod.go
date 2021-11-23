package v1alpha2

import (
	"encoding/json"
	"fmt"
	"path/filepath"
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
	SecretTemplateName                     = "secret-volume-%s"
	DefaultSecretPathName                  = "/var/build-secrets/%s"
	CosignDefaultSecretPathName            = "/var/build-secrets/cosign/%s"
	BuildLabel                             = "kpack.io/build"
	DOCKERSecretAnnotationPrefix           = "kpack.io/docker"
	GITSecretAnnotationPrefix              = "kpack.io/git"
	COSIGNDockerMediaTypesAnnotationPrefix = "kpack.io/cosign.docker-media-types"
	COSIGNRespositoryAnnotationPrefix      = "kpack.io/cosign.repository"
	COSIGNSecretDataCosignKey              = "cosign.key"
	COSIGNSecretDataCosignPassword         = "cosign.password"
	k8sOSLabel                             = "kubernetes.io/os"

	cacheDirName                 = "cache-dir"
	layersDirName                = "layers-dir"
	platformDir                  = "platform-dir"
	homeDir                      = "home-dir"
	workspaceDir                 = "workspace-dir"
	registrySourcePullSecretsDir = "registry-source-pull-secrets-dir"

	notaryDirName = "notary-dir"
	reportDirName = "report-dir"

	networkWaitLauncherDir = "network-wait-launcher-dir"

	buildChangesEnvVar = "BUILD_CHANGES"
	platformAPIEnvVar  = "CNB_PLATFORM_API"

	serviceBindingRootEnvVar = "SERVICE_BINDING_ROOT"
)

type ServiceBinding interface {
	ServiceName() string
}

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

type BuildContext struct {
	BuildPodBuilderConfig BuildPodBuilderConfig
	Secrets               []corev1.Secret
	Bindings              []ServiceBinding
	ImagePullSecrets      []corev1.LocalObjectReference
}

func (c BuildContext) os() string {
	return c.BuildPodBuilderConfig.OS
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
	registrySourcePullSecretsVolume = corev1.VolumeMount{
		Name:      registrySourcePullSecretsDir,
		MountPath: "/registrySourcePullSecrets",
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
	serviceBindingRootEnv = corev1.EnvVar{
		Name:  serviceBindingRootEnvVar,
		Value: filepath.Join(platformVolume.MountPath, "bindings"),
	}
)

type stepModifier func(corev1.Container) corev1.Container

func (b *Build) BuildPod(images BuildPodImages, buildContext BuildContext) (*corev1.Pod, error) {
	platformAPI, err := buildContext.BuildPodBuilderConfig.highestSupportedPlatformAPI(b)
	if err != nil {
		return nil, err
	}

	if b.rebasable(buildContext.BuildPodBuilderConfig.StackID) {
		return b.rebasePod(buildContext, images)
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

	secretVolumes, secretVolumeMounts, secretArgs := b.setupSecretVolumesAndArgs(buildContext.Secrets, gitAndDockerSecrets)
	cosignVolumes, cosignVolumeMounts, cosignSecretArgs := b.setupCosignVolumes(buildContext.Secrets)
	imagePullVolumes, imagePullVolumeMounts, imagePullArgs := b.setupImagePullVolumes(buildContext.ImagePullSecrets)

	bindingVolumes, bindingVolumeMounts, err := setupBindingVolumesAndMounts(buildContext.Bindings)
	if err != nil {
		return nil, err
	}

	workspaceVolume := corev1.VolumeMount{
		Name:      sourceVolume.Name,
		MountPath: sourceVolume.MountPath,
		SubPath:   b.Spec.Source.SubPath, // empty string is a nop
	}
	platformAPILessThan07 := platformAPI.LessThan(semver.MustParse("0.7"))
	var genericCacheArgs []string
	var analyzerCacheArgs []string = nil
	var exporterCacheArgs []string
	var cacheVolumes []corev1.VolumeMount

	if b.Spec.NeedVolumeCache() && buildContext.os() != "windows" {
		genericCacheArgs = []string{"-cache-dir=/cache"}
		cacheVolumes = []corev1.VolumeMount{cacheVolume}
		if platformAPILessThan07 {
			analyzerCacheArgs = genericCacheArgs
		}
		exporterCacheArgs = genericCacheArgs
	} else if b.Spec.NeedRegistryCache() {
		useCacheFromLastBuild := (b.Spec.LastBuild != nil && b.Spec.LastBuild.Cache.Image != "")
		if useCacheFromLastBuild {
			genericCacheArgs = []string{fmt.Sprintf("-cache-image=%s", b.Spec.LastBuild.Cache.Image)}
		}
		analyzerCacheArgs = genericCacheArgs
		exporterCacheArgs = []string{fmt.Sprintf("-cache-image=%s", b.Spec.Cache.Registry.Tag)}
	} else {
		genericCacheArgs = nil
	}

	analyzeContainer := corev1.Container{
		Name:      "analyze",
		Image:     b.Spec.Builder.Image,
		Command:   []string{"/cnb/lifecycle/analyzer"},
		Resources: b.Spec.Resources,
		Args: args([]string{
			"-layers=/layers",
			"-analyzed=/layers/analyzed.toml"},
			analyzerCacheArgs,
			func() []string {
				if platformAPILessThan07 {
					return []string{
						"-group=/layers/group.toml",
					}
				}
				return []string{}
			}(),
			func() []string {
				if platformAPILessThan07 {
					return []string{}
				}
				tags := []string{}
				if len(b.Spec.Tags) > 1 {
					for _, tag := range b.Spec.Tags[1:] {
						tags = append(tags, "-tag="+tag)
					}
				}
				return tags
			}(),
			func() []string {
				if b.Spec.LastBuild != nil && b.Spec.LastBuild.Image != "" {
					if platformAPILessThan07 {
						return []string{b.Spec.LastBuild.Image}
					}
					return []string{"-previous-image=" + b.Spec.LastBuild.Image, b.Tag()}
				}
				return []string{b.Tag()}
			}(),
		),
		VolumeMounts: volumeMounts([]corev1.VolumeMount{
			layersVolume,
			workspaceVolume,
			homeVolume,
		}, func() []corev1.VolumeMount {
			if platformAPILessThan07 {
				return cacheVolumes
			}
			return []corev1.VolumeMount{}
		}()),
		Env: []corev1.EnvVar{
			homeEnv,
			{
				Name:  platformAPIEnvVar,
				Value: platformAPI.Original(),
			},
			serviceBindingRootEnv,
		},
		ImagePullPolicy: corev1.PullIfNotPresent,
	}
	analyzerContainerMods := ifWindows(
		buildContext.os(),
		addNetworkWaitLauncherVolume(),
		useNetworkWaitLauncher(dnsProbeHost),
		userprofileHomeEnv(),
	)
	detectContainer := corev1.Container{
		Name:      "detect",
		Image:     b.Spec.Builder.Image,
		Command:   []string{"/cnb/lifecycle/detector"},
		Resources: b.Spec.Resources,
		Args: []string{
			"-app=/workspace",
			"-group=/layers/group.toml",
			"-plan=/layers/plan.toml",
		},
		VolumeMounts: volumeMounts([]corev1.VolumeMount{
			layersVolume,
			platformVolume,
			workspaceVolume,
		}, bindingVolumeMounts),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			{
				Name:  platformAPIEnvVar,
				Value: platformAPI.Original(),
			},
		},
	}
	detectContainerMods := ifWindows(buildContext.os(), addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))
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
				step(corev1.Container{
					Name:    "completion",
					Image:   images.completion(buildContext.os()),
					Command: []string{"/cnb/process/web"},
					Env: []corev1.EnvVar{
						homeEnv,
					},
					Args: args(
						b.notaryArgs(),
						secretArgs,
						cosignSecretArgs,
						b.cosignArgs(),
					),
					Resources: b.Spec.Resources,
					VolumeMounts: volumeMounts(
						secretVolumeMounts,
						cosignVolumeMounts,
						[]corev1.VolumeMount{
							reportVolume,
							notaryV1Volume,
							homeVolume,
						},
					),
					ImagePullPolicy: corev1.PullIfNotPresent,
				}, ifWindows(buildContext.os(), addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost), userprofileHomeEnv())...)
			}),
			SecurityContext: podSecurityContext(buildContext.BuildPodBuilderConfig),
			InitContainers: steps(func(step func(corev1.Container, ...stepModifier)) {
				step(
					corev1.Container{
						Name:      "prepare",
						Image:     images.buildInit(buildContext.os()),
						Args:      append(secretArgs, imagePullArgs...),
						Resources: b.Spec.Resources,
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
								Value: buildContext.BuildPodBuilderConfig.RunImage,
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
						VolumeMounts: volumeMounts(
							secretVolumeMounts,
							imagePullVolumeMounts,
							[]corev1.VolumeMount{
								registrySourcePullSecretsVolume,
								platformVolume,
								sourceVolume,
								homeVolume,
								projectMetadataVolume,
							},
						),
					},
					ifWindows(buildContext.os(), addNetworkWaitLauncherVolume())...,
				)
				step(
					func() corev1.Container {
						if platformAPILessThan07 {
							return detectContainer
						}
						return analyzeContainer
					}(),
					func() []stepModifier {
						if platformAPILessThan07 {
							return detectContainerMods
						}
						return analyzerContainerMods
					}()...,
				)
				step(
					func() corev1.Container {
						if platformAPILessThan07 {
							return analyzeContainer
						}
						return detectContainer
					}(),
					func() []stepModifier {
						if platformAPILessThan07 {
							return analyzerContainerMods
						}
						return detectContainerMods
					}()...,
				)
				step(
					corev1.Container{
						Name:      "restore",
						Image:     b.Spec.Builder.Image,
						Command:   []string{"/cnb/lifecycle/restorer"},
						Resources: b.Spec.Resources,
						Args: args([]string{
							"-group=/layers/group.toml",
							"-layers=/layers",
						}, genericCacheArgs, func() []string {
							if platformAPILessThan07 {
								return []string{}
							}
							return []string{"-analyzed=/layers/analyzed.toml"}
						}()),
						VolumeMounts: volumeMounts([]corev1.VolumeMount{
							layersVolume,
							homeVolume,
						}, cacheVolumes),
						Env: []corev1.EnvVar{
							homeEnv,
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
					ifWindows(buildContext.os(),
						addNetworkWaitLauncherVolume(),
						useNetworkWaitLauncher(dnsProbeHost),
						userprofileHomeEnv(),
					)...,
				)
				step(
					corev1.Container{
						Name:      "build",
						Image:     b.Spec.Builder.Image,
						Command:   []string{"/cnb/lifecycle/builder"},
						Resources: b.Spec.Resources,
						Args: []string{
							"-layers=/layers",
							"-app=/workspace",
							"-group=/layers/group.toml",
							"-plan=/layers/plan.toml",
						},
						VolumeMounts: volumeMounts([]corev1.VolumeMount{
							layersVolume,
							platformVolume,
							workspaceVolume,
						}, bindingVolumeMounts),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
							serviceBindingRootEnv,
						},
					},
					ifWindows(buildContext.os(), addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))...,
				)
				step(
					corev1.Container{
						Name:      "export",
						Image:     b.Spec.Builder.Image,
						Command:   []string{"/cnb/lifecycle/exporter"},
						Resources: b.Spec.Resources,
						Args: args([]string{
							"-layers=/layers",
							"-app=/workspace",
							"-group=/layers/group.toml",
							"-analyzed=/layers/analyzed.toml",
							"-project-metadata=/layers/project-metadata.toml"},
							exporterCacheArgs,
							func() []string {
								if b.DefaultProcess() == "" {
									if platformAPI.Equal(lowestSupportedPlatformVersion) || platformAPI.GreaterThan(semver.MustParse("0.5")) {
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
						VolumeMounts: volumeMounts([]corev1.VolumeMount{
							layersVolume,
							workspaceVolume,
							homeVolume,
							reportVolume,
						}, cacheVolumes),
						Env: []corev1.EnvVar{
							homeEnv,
							{
								Name:  platformAPIEnvVar,
								Value: platformAPI.Original(),
							},
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
					},
					ifWindows(buildContext.os(),
						addNetworkWaitLauncherVolume(),
						useNetworkWaitLauncher(dnsProbeHost),
						userprofileHomeEnv(),
					)...,
				)
			}),
			ServiceAccountName: b.Spec.ServiceAccountName,
			NodeSelector:       b.nodeSelector(buildContext.os()),
			Tolerations:        b.Spec.Tolerations,
			Affinity:           b.Spec.Affinity,
			RuntimeClassName:   b.Spec.RuntimeClassName,
			SchedulerName:      b.Spec.SchedulerName,
			Volumes: volumes(
				secretVolumes,
				cosignVolumes,
				imagePullVolumes,
				b.cacheVolume(buildContext.os()),
				[]corev1.Volume{
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
					{
						Name: reportDirName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: networkWaitLauncherDir,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					b.Spec.Source.Source().ImagePullSecretsVolume(registrySourcePullSecretsDir),
					b.notarySecretVolume(),
				},
				bindingVolumes),
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

func (b *Build) notaryArgs() []string {
	if b.NotaryV1Config() == nil {
		return nil
	}
	return []string{"-notary-v1-url=" + b.NotaryV1Config().URL}
}

func (b *Build) cosignArgs() []string {
	args := []string{
		fmt.Sprintf("-cosign-annotations=buildTimestamp=%s", b.ObjectMeta.CreationTimestamp.Format("20060102.150405")),
		fmt.Sprintf("-cosign-annotations=buildNumber=%s", b.Labels[BuildNumberLabel]),
	}

	if b.Spec.Cosign != nil && b.Spec.Cosign.Annotations != nil {
		for _, annotation := range b.Spec.Cosign.Annotations {
			args = append(args, fmt.Sprintf("-cosign-annotations=%s=%s", annotation.Name, annotation.Value))
		}
	}
	return args
}

func (b *Build) rebasePod(buildContext BuildContext, images BuildPodImages) (*corev1.Pod, error) {
	secretVolumes, secretVolumeMounts, secretArgs := b.setupSecretVolumesAndArgs(buildContext.Secrets, dockerSecrets)
	cosignVolumes, cosignVolumeMounts, cosignSecretArgs := b.setupCosignVolumes(buildContext.Secrets)

	imagePullVolumes, imagePullVolumeMounts, imagePullArgs := b.setupImagePullVolumes(buildContext.ImagePullSecrets)

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
			ServiceAccountName: b.Spec.ServiceAccountName,
			NodeSelector:       b.nodeSelector("linux"),
			Tolerations:        b.Spec.Tolerations,
			Affinity:           b.Spec.Affinity,
			RuntimeClassName:   b.Spec.RuntimeClassName,
			SchedulerName:      b.Spec.SchedulerName,
			Volumes: volumes(
				secretVolumes,
				cosignVolumes,
				imagePullVolumes,
				[]corev1.Volume{
					{
						Name: reportDirName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					b.notarySecretVolume(),
				},
			),
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "completion",
					Image:   images.completion(buildContext.os()),
					Command: []string{"/cnb/process/web"},
					Args: args(
						b.notaryArgs(),
						secretArgs,
						b.cosignArgs(),
						cosignSecretArgs,
					),
					Resources: b.Spec.Resources,
					VolumeMounts: volumeMounts(
						secretVolumeMounts,
						cosignVolumeMounts,
						[]corev1.VolumeMount{
							reportVolume,
							notaryV1Volume,
						},
					),
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:      "rebase",
					Image:     images.RebaseImage,
					Resources: b.Spec.Resources,
					Args: args(a(
						"--run-image",
						buildContext.BuildPodBuilderConfig.RunImage,
						"--last-built-image",
						b.Spec.LastBuild.Image,
						"--report",
						"/var/report/report.toml",
					),
						secretArgs,
						imagePullArgs,
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
					VolumeMounts: volumeMounts(
						secretVolumeMounts,
						imagePullVolumeMounts,
						[]corev1.VolumeMount{
							reportVolume,
						},
					),
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
			MountPath: fmt.Sprintf(DefaultSecretPathName, secret.Name),
		})
	}

	return volumes, volumeMounts, args
}

func (b *Build) setupImagePullVolumes(secrets []corev1.LocalObjectReference) ([]corev1.Volume, []corev1.VolumeMount, []string) {
	var (
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		args         []string
	)
	for _, secret := range deduplicate(secrets, b.Spec.Builder.ImagePullSecrets) {
		args = append(args, fmt.Sprintf("-imagepull=%s", secret.Name))
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
			MountPath: fmt.Sprintf(DefaultSecretPathName, secret.Name),
		})
	}

	return volumes, volumeMounts, args
}

func (b *Build) setupCosignVolumes(secrets []corev1.Secret) ([]corev1.Volume, []corev1.VolumeMount, []string) {
	var (
		volumes      []corev1.Volume
		volumeMounts []corev1.VolumeMount
		args         []string
	)
	for _, secret := range secrets {
		if string(secret.Data[COSIGNSecretDataCosignKey]) == "" {
			continue
		}

		cosignArgs := cosignSecretArgs(secret)
		args = append(args, cosignArgs...)

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
			MountPath: fmt.Sprintf(CosignDefaultSecretPathName, secret.Name),
		})
	}

	return volumes, volumeMounts, args
}

var (
	lowestSupportedPlatformVersion = semver.MustParse("0.3")

	supportedPlatformAPIVersionsWithWindowsAndReportToml = []*semver.Version{semver.MustParse("0.8"), semver.MustParse("0.7"), semver.MustParse("0.6"), semver.MustParse("0.5"), semver.MustParse("0.4")}
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

func (b Build) nodeSelector(os string) map[string]string {
	if b.Spec.NodeSelector == nil {
		b.Spec.NodeSelector = map[string]string{}
	}

	b.Spec.NodeSelector[k8sOSLabel] = os
	return b.Spec.NodeSelector
}

func setupBindingVolumesAndMounts(bindings []ServiceBinding) ([]corev1.Volume, []corev1.VolumeMount, error) {
	volumes := make([]corev1.Volume, 0, len(bindings))
	volumeMounts := make([]corev1.VolumeMount, 0, len(bindings))

	for _, binding := range bindings {
		switch b := binding.(type) {
		case *corev1alpha1.ServiceBinding:
			if b.SecretRef != nil {
				secretVolume := fmt.Sprintf("service-binding-secret-%s", b.Name)
				volumes = append(volumes,
					corev1.Volume{
						Name: secretVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: b.SecretRef.Name,
							},
						},
					},
				)
				volumeMounts = append(volumeMounts,
					corev1.VolumeMount{
						Name:      secretVolume,
						MountPath: fmt.Sprintf("%s/bindings/%s", platformVolume.MountPath, b.Name),
						ReadOnly:  true,
					},
				)
			}
		case *corev1alpha1.CNBServiceBinding:
			metadataVolume := fmt.Sprintf("binding-metadata-%s", b.Name)
			volumes = append(volumes,
				corev1.Volume{
					Name: metadataVolume,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: *b.MetadataRef,
						},
					},
				},
			)
			volumeMounts = append(volumeMounts,
				corev1.VolumeMount{
					Name:      metadataVolume,
					MountPath: fmt.Sprintf("%s/bindings/%s/metadata", platformVolume.MountPath, b.Name),
					ReadOnly:  true,
				},
			)
			if b.SecretRef != nil {
				secretVolume := fmt.Sprintf("binding-secret-%s", b.Name)
				volumes = append(volumes,
					corev1.Volume{
						Name: secretVolume,
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: b.SecretRef.Name,
							},
						},
					},
				)
				volumeMounts = append(volumeMounts,
					corev1.VolumeMount{
						Name:      secretVolume,
						MountPath: fmt.Sprintf("%s/bindings/%s/secret", platformVolume.MountPath, b.Name),
						ReadOnly:  true,
					},
				)
			}
		default:
			return nil, nil, errors.Errorf("unsupported binding type: %T", b)
		}
	}

	return volumes, volumeMounts, nil
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

func volumes(volumes ...[]corev1.Volume) []corev1.Volume {
	var combined []corev1.Volume
	for _, v := range volumes {
		combined = append(combined, v...)
	}
	return combined
}

func volumeMounts(volumes ...[]corev1.VolumeMount) []corev1.VolumeMount {
	var combined []corev1.VolumeMount
	for _, v := range volumes {
		combined = append(combined, v...)
	}
	return combined
}

func deduplicate(lists ...[]corev1.LocalObjectReference) []corev1.LocalObjectReference {
	names := map[string]struct{}{}
	var deduplicated []corev1.LocalObjectReference

	for _, list := range lists {
		for _, entry := range list {
			if _, ok := names[entry.Name]; !ok {
				deduplicated = append(deduplicated, entry)
			}
			names[entry.Name] = struct{}{}
		}
	}
	return deduplicated
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

func cosignSecretArgs(secret corev1.Secret) []string {
	var cosignArgs []string
	if cosignRepository := secret.ObjectMeta.Annotations[COSIGNRespositoryAnnotationPrefix]; cosignRepository != "" {
		cosignArgs = append(cosignArgs, fmt.Sprintf("-cosign-repositories=%s=%s", secret.Name, cosignRepository))
	}

	if cosignDockerMediaType := secret.ObjectMeta.Annotations[COSIGNDockerMediaTypesAnnotationPrefix]; cosignDockerMediaType != "" {
		cosignArgs = append(cosignArgs, fmt.Sprintf("-cosign-docker-media-types=%s=%s", secret.Name, cosignDockerMediaType))
	}

	return cosignArgs
}
