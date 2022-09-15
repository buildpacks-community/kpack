package v1alpha2

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	PrepareContainerName    = "prepare"
	AnalyzeContainerName    = "analyze"
	DetectContainerName     = "detect"
	RestoreContainerName    = "restore"
	BuildContainerName      = "build"
	ExportContainerName     = "export"
	RebaseContainerName     = "rebase"
	CompletionContainerName = "completion"

	secretVolumeNameTemplate     = "secret-volume-%v"
	pullSecretVolumeNameTemplate = "pull-secret-volume-%v"

	completionTerminationMessagePathWindows = "/dev/termination-log"
	completionTerminationMessagePathLinux   = "/tmp/termination-log"
	cosignDefaultSecretPath                 = "/var/build-secrets/cosign/%s"
	defaultSecretPath                       = "/var/build-secrets/%s"
	ReportTOMLPath                          = "/var/report/report.toml"

	BuildLabel = "kpack.io/build"
	k8sOSLabel = "kubernetes.io/os"

	cosignDockerMediaTypesAnnotationPrefix = "kpack.io/cosign.docker-media-types"
	cosignRespositoryAnnotationPrefix      = "kpack.io/cosign.repository"
	DOCKERSecretAnnotationPrefix           = "kpack.io/docker"
	GITSecretAnnotationPrefix              = "kpack.io/git"
	IstioInject                            = "sidecar.istio.io/inject"

	cosignSecretDataCosignKey = "cosign.key"

	cacheVolumeName                     = "cache-dir"
	homeVolumeName                      = "home-dir"
	layersVolumeName                    = "layers-dir"
	networkWaitLauncherVolumeName       = "network-wait-launcher-dir"
	notaryVolumeName                    = "notary-dir"
	platformVolumeName                  = "platform-dir"
	registrySourcePullSecretsVolumeName = "registry-source-pull-secrets-dir"
	reportVolumeName                    = "report-dir"
	workspaceVolumeName                 = "workspace-dir"

	buildChangesEnvVar           = "BUILD_CHANGES"
	CacheTagEnvVar               = "CACHE_TAG"
	platformApiVersionEnvVarName = "CNB_PLATFORM_API"
	serviceBindingRootEnvVar     = "SERVICE_BINDING_ROOT"
	TerminationMessagePathEnvVar = "TERMINATION_MESSAGE_PATH"

	PlatformEnvVarPrefix = "PLATFORM_ENV_"
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

func terminationMsgPath(os string) string {
	switch os {
	case "windows":
		return completionTerminationMessagePathWindows
	default:
		return completionTerminationMessagePathLinux
	}
}

// +k8s:deepcopy-gen=false
type BuildContext struct {
	BuildPodBuilderConfig     BuildPodBuilderConfig
	Secrets                   []corev1.Secret
	Bindings                  []ServiceBinding
	ImagePullSecrets          []corev1.LocalObjectReference
	MaximumPlatformApiVersion *semver.Version
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
	sourceMount = corev1.VolumeMount{
		Name:      workspaceVolumeName,
		MountPath: "/workspace",
	}
	homeMount = corev1.VolumeMount{
		Name:      homeVolumeName,
		MountPath: "/builder/home",
	}
	platformMount = corev1.VolumeMount{
		Name:      platformVolumeName,
		MountPath: "/platform",
	}
	cacheMount = corev1.VolumeMount{
		Name:      cacheVolumeName,
		MountPath: "/cache",
	}
	layersMount = corev1.VolumeMount{
		Name:      layersVolumeName,
		MountPath: "/layers",
	}
	projectMetadataMount = corev1.VolumeMount{
		Name:      layersVolumeName,
		MountPath: "/projectMetadata",
	}
	registrySourcePullSecretsMount = corev1.VolumeMount{
		Name:      registrySourcePullSecretsVolumeName,
		MountPath: "/registrySourcePullSecrets",
		ReadOnly:  true,
	}
	notaryV1Mount = corev1.VolumeMount{
		Name:      notaryVolumeName,
		MountPath: "/var/notary/v1",
		ReadOnly:  true,
	}
	reportMount = corev1.VolumeMount{
		Name:      reportVolumeName,
		MountPath: "/var/report",
		ReadOnly:  false,
	}
	networkWaitLauncherMount = corev1.VolumeMount{
		Name:      networkWaitLauncherVolumeName,
		MountPath: "/networkWait",
		ReadOnly:  false,
	}
	homeEnv = corev1.EnvVar{
		Name:  "HOME",
		Value: "/builder/home",
	}
	serviceBindingRootEnv = corev1.EnvVar{
		Name:  serviceBindingRootEnvVar,
		Value: filepath.Join(platformMount.MountPath, "bindings"),
	}
)

type stepModifier func(corev1.Container) corev1.Container

func (b *Build) BuildPod(images BuildPodImages, buildContext BuildContext) (*corev1.Pod, error) {
	platformAPI, err := buildContext.highestSupportedPlatformAPI(b)
	if err != nil {
		return nil, err
	}
	platformApiVersionEnvVar := corev1.EnvVar{Name: platformApiVersionEnvVarName, Value: platformAPI.Original()}

	if b.rebasable(buildContext.BuildPodBuilderConfig.StackID) {
		return b.rebasePod(buildContext, images)
	}

	ref, err := name.ParseReference(b.Tag())
	if err != nil {
		return nil, err
	}
	dnsProbeHost := ref.Context().RegistryStr()

	buildEnv := b.Spec.Source.Source().BuildEnvVars()
	for _, envVar := range b.Spec.Env {
		envVar.Name = PlatformEnvVarPrefix + envVar.Name
		buildEnv = append(buildEnv, envVar)
	}

	secretVolumes, secretVolumeMounts, secretArgs := b.setupSecretVolumesAndArgs(buildContext.Secrets, gitAndDockerSecrets)
	cosignVolumes, cosignVolumeMounts, cosignSecretArgs := b.setupCosignVolumes(buildContext.Secrets)
	imagePullVolumes, imagePullVolumeMounts, imagePullArgs := b.setupImagePullVolumes(buildContext.ImagePullSecrets)

	bindingVolumes, bindingVolumeMounts, err := setupBindingVolumesAndMounts(buildContext.Bindings)
	if err != nil {
		return nil, err
	}

	runImage := buildContext.BuildPodBuilderConfig.RunImage
	if b.Spec.RunImage.Image != "" {
		runImage = b.Spec.RunImage.Image
	}

	workspaceVolume := corev1.VolumeMount{
		Name:      sourceMount.Name,
		MountPath: sourceMount.MountPath,
		SubPath:   b.Spec.Source.SubPath, // empty string is a nop
	}
	platformAPILessThan07 := platformAPI.LessThan(semver.MustParse("0.7"))
	var genericCacheArgs []string
	var analyzerCacheArgs []string = nil
	var exporterCacheArgs []string
	var cacheVolumes []corev1.VolumeMount

	if b.Spec.NeedVolumeCache() && buildContext.os() != "windows" {
		genericCacheArgs = []string{"-cache-dir=/cache"}
		cacheVolumes = []corev1.VolumeMount{cacheMount}
		if platformAPILessThan07 {
			analyzerCacheArgs = genericCacheArgs
		}
		exporterCacheArgs = genericCacheArgs
	} else if b.Spec.NeedRegistryCache() {
		useCacheFromLastBuild := b.Spec.LastBuild != nil && b.Spec.LastBuild.Cache.Image != ""
		if useCacheFromLastBuild {
			genericCacheArgs = []string{fmt.Sprintf("-cache-image=%s", b.Spec.LastBuild.Cache.Image)}
		}
		analyzerCacheArgs = genericCacheArgs
		exporterCacheArgs = []string{fmt.Sprintf("-cache-image=%s", b.Spec.Cache.Registry.Tag)}
	} else {
		genericCacheArgs = nil
	}

	analyzeContainer := corev1.Container{
		Name:      AnalyzeContainerName,
		Image:     b.Spec.Builder.Image,
		Command:   []string{"/cnb/lifecycle/analyzer"},
		Resources: b.Spec.Resources,
		Args: args(
			[]string{"-layers=/layers", "-analyzed=/layers/analyzed.toml"},
			func() []string {
				if !platformAPILessThan07 {
					return []string{"-run-image=" + runImage}
				}
				return []string{}
			}(),
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
		SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
		VolumeMounts: volumeMounts([]corev1.VolumeMount{
			layersMount,
			workspaceVolume,
			homeMount,
		}, func() []corev1.VolumeMount {
			if platformAPILessThan07 {
				return cacheVolumes
			}
			return []corev1.VolumeMount{}
		}()),
		Env: []corev1.EnvVar{
			homeEnv,
			platformApiVersionEnvVar,
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
		Name:      DetectContainerName,
		Image:     b.Spec.Builder.Image,
		Command:   []string{"/cnb/lifecycle/detector"},
		Resources: b.Spec.Resources,
		Args: []string{
			"-app=/workspace",
			"-group=/layers/group.toml",
			"-plan=/layers/plan.toml",
		},
		VolumeMounts: volumeMounts([]corev1.VolumeMount{
			layersMount,
			platformMount,
			workspaceVolume,
		}, bindingVolumeMounts),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env: []corev1.EnvVar{
			platformApiVersionEnvVar,
		},
		SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
	}
	detectContainerMods := ifWindows(buildContext.os(), addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))

	dateTime, err := parseTime(b.Spec.CreationTime)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing creation time %s", b.Spec.CreationTime)
	}
	
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.PodName(),
			Namespace: b.Namespace,
			Labels: combine(b.Labels, map[string]string{
				BuildLabel: b.Name,
			}),
			Annotations: combine(b.Annotations, map[string]string{
				IstioInject: "false",
			}),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
		},
		Spec: corev1.PodSpec{
			// If the build fails, don't restart it.
			RestartPolicy:         corev1.RestartPolicyNever,
			ActiveDeadlineSeconds: b.Spec.ActiveDeadlineSeconds,
			PriorityClassName:     b.PriorityClassName(),
			Containers: steps(func(step func(corev1.Container, ...stepModifier)) {
				step(
					corev1.Container{
						Name:    CompletionContainerName,
						Image:   images.completion(buildContext.os()),
						Command: []string{"/cnb/process/completion"},
						Env: []corev1.EnvVar{
							homeEnv,
							{Name: CacheTagEnvVar, Value: b.Spec.RegistryCacheTag()},
							{Name: TerminationMessagePathEnvVar, Value: terminationMsgPath(buildContext.os())},
						},
						Args: args(
							b.notaryArgs(),
							secretArgs,
							b.cosignArgs(),
							cosignSecretArgs,
						),
						TerminationMessagePath:   terminationMsgPath(buildContext.os()),
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Resources:                b.Spec.Resources,
						VolumeMounts: volumeMounts(
							secretVolumeMounts,
							cosignVolumeMounts,
							[]corev1.VolumeMount{
								homeMount,
								reportMount,
								notaryV1Mount,
							},
						),
						ImagePullPolicy: corev1.PullIfNotPresent,
						SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
					},
					ifWindows(buildContext.os(), addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost), userprofileHomeEnv())...)
			}),
			SecurityContext: podSecurityContext(buildContext.BuildPodBuilderConfig),
			InitContainers: steps(func(step func(corev1.Container, ...stepModifier)) {
				step(
					corev1.Container{
						Name:            PrepareContainerName,
						Image:           images.buildInit(buildContext.os()),
						Args:            append(secretArgs, imagePullArgs...),
						Resources:       b.Spec.Resources,
						SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
						Env: append(
							buildEnv,
							corev1.EnvVar{
								Name:  "SOURCE_SUB_PATH",
								Value: b.Spec.Source.SubPath,
							},
							corev1.EnvVar{
								Name:  "PROJECT_DESCRIPTOR_PATH",
								Value: b.Spec.ProjectDescriptorPath,
							},
							corev1.EnvVar{
								Name:  "IMAGE_TAG",
								Value: b.Tag(),
							},
							corev1.EnvVar{
								Name:  "RUN_IMAGE",
								Value: runImage,
							},
							corev1.EnvVar{
								Name:  "BUILDER_IMAGE",
								Value: b.BuilderSpec().Image,
							},
							corev1.EnvVar{
								Name:  "BUILDER_NAME",
								Value: b.builderName(),
							},
							corev1.EnvVar{
								Name:  "BUILDER_KIND",
								Value: b.builderKind(),
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
								registrySourcePullSecretsMount,
								platformMount,
								sourceMount,
								homeMount,
								projectMetadataMount,
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
						Name:            RestoreContainerName,
						Image:           b.Spec.Builder.Image,
						Command:         []string{"/cnb/lifecycle/restorer"},
						Resources:       b.Spec.Resources,
						SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
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
							layersMount,
							homeMount,
						}, cacheVolumes),
						Env: []corev1.EnvVar{
							homeEnv,
							platformApiVersionEnvVar,
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
						Name:            BuildContainerName,
						Image:           b.Spec.Builder.Image,
						Command:         []string{"/cnb/lifecycle/builder"},
						Resources:       b.Spec.Resources,
						SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
						Args: []string{
							"-layers=/layers",
							"-app=/workspace",
							"-group=/layers/group.toml",
							"-plan=/layers/plan.toml",
						},
						VolumeMounts: volumeMounts([]corev1.VolumeMount{
							layersMount,
							platformMount,
							workspaceVolume,
						}, bindingVolumeMounts),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Env: []corev1.EnvVar{
							platformApiVersionEnvVar,
							serviceBindingRootEnv,
						},
					},
					ifWindows(buildContext.os(), addNetworkWaitLauncherVolume(), useNetworkWaitLauncher(dnsProbeHost))...,
				)
				step(
					corev1.Container{
						Name:            ExportContainerName,
						Image:           b.Spec.Builder.Image,
						Command:         []string{"/cnb/lifecycle/exporter"},
						Resources:       b.Spec.Resources,
						SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
						Args: args(
							[]string{
								"-layers=/layers",
								"-app=/workspace",
								"-group=/layers/group.toml",
								"-analyzed=/layers/analyzed.toml",
								"-project-metadata=/layers/project-metadata.toml",
							},
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
								return []string{fmt.Sprintf("-report=%s", ReportTOMLPath)}

							}(),
							func() []string {
								if platformAPILessThan07 {
									return []string{"-run-image=" + runImage}
								}
								return []string{}
							}(),
							b.Spec.Tags),
						VolumeMounts: volumeMounts([]corev1.VolumeMount{
							layersMount,
							workspaceVolume,
							homeMount,
							reportMount,
						}, cacheVolumes),
						Env: envs(
							[]corev1.EnvVar{
								homeEnv,
								platformApiVersionEnvVar,
							},
							func() corev1.EnvVar {
								if dateTime != nil {
									return corev1.EnvVar{Name: "SOURCE_DATE_EPOCH", Value: strconv.Itoa(int(dateTime.Unix()))}
								}
								return corev1.EnvVar{Name:"", Value:""}
							}(),
							func() corev1.EnvVar {
								return corev1.EnvVar{
									Name:  "CNB_RUN_IMAGE",
									Value: runImage,
								}
							}()),
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
						Name: layersVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: homeVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: workspaceVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: platformVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: reportVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: networkWaitLauncherVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					b.Spec.Source.Source().ImagePullSecretsVolume(registrySourcePullSecretsVolumeName),
					b.notarySecretVolume(),
				},
				bindingVolumes),
			ImagePullSecrets: b.Spec.Builder.ImagePullSecrets,
		},
	}, nil
}

func boolPointer(b bool) *bool {
	return &b
}

func containerSecurityContext(config BuildPodBuilderConfig) *corev1.SecurityContext {
	if config.OS == "windows" {
		return nil

	}

	return &corev1.SecurityContext{
		RunAsNonRoot:             boolPointer(true),
		AllowPrivilegeEscalation: boolPointer(false),
		Privileged:               boolPointer(false),
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
}

func podSecurityContext(config BuildPodBuilderConfig) *corev1.PodSecurityContext {
	if config.OS == "windows" {
		return nil
	}

	return &corev1.PodSecurityContext{
		RunAsNonRoot:   boolPointer(true),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		FSGroup:        &config.Gid,
		RunAsUser:      &config.Uid,
		RunAsGroup:     &config.Gid,
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
		container.VolumeMounts = append(container.VolumeMounts, networkWaitLauncherMount)
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
			Name: notaryVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}

	}

	return corev1.Volume{
		Name: notaryVolumeName,
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
	runImage := buildContext.BuildPodBuilderConfig.RunImage
	if b.Spec.RunImage.Image != "" {
		runImage = b.Spec.RunImage.Image
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.PodName(),
			Namespace: b.Namespace,
			Labels: combine(b.Labels, map[string]string{
				BuildLabel: b.Name,
			}),
			Annotations: combine(b.Annotations, map[string]string{
				IstioInject: "false",
			}),
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
			PriorityClassName:  b.PriorityClassName(),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   boolPointer(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Volumes: volumes(
				secretVolumes,
				cosignVolumes,
				imagePullVolumes,
				[]corev1.Volume{
					{
						Name: reportVolumeName,
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
					Name:    CompletionContainerName,
					Image:   images.completion(buildContext.os()),
					Command: []string{"/cnb/process/completion"},
					Env: []corev1.EnvVar{
						{Name: CacheTagEnvVar, Value: b.Spec.RegistryCacheTag()},
						{Name: TerminationMessagePathEnvVar, Value: terminationMsgPath(buildContext.os())},
					},
					Args: args(
						b.notaryArgs(),
						secretArgs,
						b.cosignArgs(),
						cosignSecretArgs,
					),
					SecurityContext:          containerSecurityContext(buildContext.BuildPodBuilderConfig),
					TerminationMessagePath:   terminationMsgPath(buildContext.os()),
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
					Resources:                b.Spec.Resources,
					VolumeMounts: volumeMounts(
						[]corev1.VolumeMount{reportMount, notaryV1Mount},
						secretVolumeMounts,
						cosignVolumeMounts,
					),
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            RebaseContainerName,
					Image:           images.RebaseImage,
					Resources:       b.Spec.Resources,
					SecurityContext: containerSecurityContext(buildContext.BuildPodBuilderConfig),
					Args: args(a(
						"--run-image",
						runImage,
						"--last-built-image",
						b.Spec.LastBuild.Image,
						"--report",
						ReportTOMLPath,
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
							reportMount,
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
		Name: cacheVolumeName,
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
	for i, secret := range secrets {
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

		volumeName := fmt.Sprintf(secretVolumeNameTemplate, i)
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
			MountPath: fmt.Sprintf(defaultSecretPath, secret.Name),
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
	for i, secret := range deduplicate(secrets, b.Spec.Builder.ImagePullSecrets) {
		args = append(args, fmt.Sprintf("-imagepull=%s", secret.Name))

		volumeName := fmt.Sprintf(pullSecretVolumeNameTemplate, i)
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
			MountPath: fmt.Sprintf(defaultSecretPath, secret.Name),
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
	for i, secret := range secrets {
		if string(secret.Data[cosignSecretDataCosignKey]) == "" {
			continue
		}

		cosignArgs := cosignSecretArgs(secret)
		args = append(args, cosignArgs...)

		volumeName := fmt.Sprintf(secretVolumeNameTemplate, i)
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
			MountPath: fmt.Sprintf(cosignDefaultSecretPath, secret.Name),
		})
	}

	return volumes, volumeMounts, args
}

var (
	lowestSupportedPlatformVersion = semver.MustParse("0.3")

	supportedPlatformAPIVersionsWithWindowsAndReportToml = []*semver.Version{semver.MustParse("0.9"), semver.MustParse("0.8"), semver.MustParse("0.7"), semver.MustParse("0.6"), semver.MustParse("0.5"), semver.MustParse("0.4")}
	supportedPlatformAPIVersions                         = append(supportedPlatformAPIVersionsWithWindowsAndReportToml, semver.MustParse("0.3"))
)

func (bc BuildContext) highestSupportedPlatformAPI(b *Build) (*semver.Version, error) {
	for _, supportedVersion := range func() []*semver.Version {
		if b.NotaryV1Config() != nil || bc.BuildPodBuilderConfig.OS == "windows" {
			return supportedPlatformAPIVersionsWithWindowsAndReportToml
		}
		return supportedPlatformAPIVersions
	}() {
		if bc.MaximumPlatformApiVersion != nil && bc.MaximumPlatformApiVersion.LessThan(supportedVersion) {
			continue
		}
		for _, v := range bc.BuildPodBuilderConfig.PlatformAPIs {
			version, err := semver.NewVersion(v)
			if err != nil {
				return nil, errors.Wrapf(err, "unexpected platform version %s", v)
			}

			if supportedVersion.Equal(version) {
				return version, nil
			}
		}
	}

	return nil, errors.Errorf("unsupported builder platform API versions: %s", strings.Join(bc.BuildPodBuilderConfig.PlatformAPIs, ","))
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
						MountPath: fmt.Sprintf("%s/bindings/%s", platformMount.MountPath, b.Name),
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
					MountPath: fmt.Sprintf("%s/bindings/%s/metadata", platformMount.MountPath, b.Name),
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
						MountPath: fmt.Sprintf("%s/bindings/%s/secret", platformMount.MountPath, b.Name),
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
	if cosignRepository := secret.ObjectMeta.Annotations[cosignRespositoryAnnotationPrefix]; cosignRepository != "" {
		cosignArgs = append(cosignArgs, fmt.Sprintf("-cosign-repositories=%s=%s", secret.Name, cosignRepository))
	}

	if cosignDockerMediaType := secret.ObjectMeta.Annotations[cosignDockerMediaTypesAnnotationPrefix]; cosignDockerMediaType != "" {
		cosignArgs = append(cosignArgs, fmt.Sprintf("-cosign-docker-media-types=%s=%s", secret.Name, cosignDockerMediaType))
	}

	return cosignArgs
}

func envs(envs []corev1.EnvVar, envVars ...corev1.EnvVar) []corev1.EnvVar {
	for _, envVar := range envVars {
		if envVar.Name != "" && envVar.Value != "" {
			envs = append(envs, envVar)
		}
	}
	return envs
}


func parseTime(providedTime string) (*time.Time, error) {
	var parsedTime time.Time
	switch providedTime {
	case "":
		return nil, nil
	case "now":
		parsedTime = time.Now().UTC()
	default:
		intTime, err := strconv.ParseInt(providedTime, 10, 64)
		if err != nil {
			return nil, errors.Wrap(err, "parsing unix timestamp")
		}
		parsedTime = time.Unix(intTime, 0).UTC()
	}
	return &parsedTime, nil
}
