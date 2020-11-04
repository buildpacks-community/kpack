package v1alpha1_test

import (
	"fmt"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func TestBuildPod(t *testing.T) {
	spec.Run(t, "Test Build Pod", testBuildPod)
}

func testBuildPod(t *testing.T, when spec.G, it spec.S) {
	const (
		directExecute    = "--"
		namespace        = "some-namespace"
		buildName        = "build-name"
		builderImage     = "builderregistry.io/builder:latest@sha256:42lkajdsf9q87234"
		previousAppImage = "someimage/name@sha256:previous"
		serviceAccount   = "someserviceaccount"
	)
	resources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2"),
			corev1.ResourceMemory: resource.MustParse("256M"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("128M"),
		},
	}

	builderImageRef := v1alpha1.BuildBuilderSpec{
		Image: builderImage,
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "some-image-secret"},
		},
	}

	build := &v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: namespace,
			Labels: map[string]string{
				"some/label": "to-pass-through",
			},
			Annotations: map[string]string{
				"some/annotation": "to-pass-through",
			},
		},
		Spec: v1alpha1.BuildSpec{
			Tags:           []string{"someimage/name", "someimage/name:tag2", "someimage/name:tag3"},
			Builder:        builderImageRef,
			ServiceAccount: serviceAccount,
			Source: v1alpha1.SourceConfig{
				Git: &v1alpha1.Git{
					URL:      "giturl.com/git.git",
					Revision: "gitrev1234",
				},
			},
			CacheName: "some-cache-name",
			Bindings: []v1alpha1.Binding{
				{
					Name: "database",
					MetadataRef: &corev1.LocalObjectReference{
						Name: "database-configmap",
					},
				},
				{
					Name: "apm",
					MetadataRef: &corev1.LocalObjectReference{
						Name: "apm-configmap",
					},
					SecretRef: &corev1.LocalObjectReference{
						Name: "apm-secret",
					},
				},
			},
			Env: []corev1.EnvVar{
				{Name: "keyA", Value: "valueA"},
				{Name: "keyB", Value: "valueB"},
			},
			Resources: resources,
			LastBuild: &v1alpha1.LastBuild{
				Image:   previousAppImage,
				StackId: "com.builder.stack.io",
			},
		},
	}

	secrets := []corev1.Secret{
		{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-secret-1",
				Annotations: map[string]string{
					v1alpha1.GITSecretAnnotationPrefix: "https://github.com",
				},
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		},
		{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-secret-2",
				Annotations: map[string]string{
					v1alpha1.GITSecretAnnotationPrefix: "https://bitbucket.com",
				},
			},
			StringData: map[string]string{
				"ssh-privatekey": "some key",
			},
			Type: corev1.SecretTypeSSHAuth,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "docker-secret-1",
				Annotations: map[string]string{
					v1alpha1.DOCKERSecretAnnotationPrefix: "acr.io",
				},
			},
			Type: corev1.SecretTypeBasicAuth,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "random-secret-1",
			},
			Type: corev1.SecretTypeBasicAuth,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "docker-secret-2",
			},
			Type: corev1.SecretTypeDockerConfigJson,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "docker-secret-3",
			},
			Type: corev1.SecretTypeDockercfg,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "secret-to-ignore",
				Annotations: map[string]string{
					v1alpha1.DOCKERSecretAnnotationPrefix: "ignoreme.com",
				},
			},
			Type: corev1.SecretTypeBootstrapToken,
		},
	}

	config := v1alpha1.BuildPodImages{
		BuildInitImage:  "build/init:image",
		CompletionImage: "completion/image:image",
	}

	buildPodBuilderConfig := v1alpha1.BuildPodBuilderConfig{
		StackID:     "com.builder.stack.io",
		RunImage:    "builderregistry.io/run",
		Uid:         2000,
		Gid:         3000,
		PlatformAPI: "0.2",
	}

	when("BuildPod", func() {
		when(">= 0.2 platform api", func() {
			it("creates a pod with a builder owner reference and build labels and annotations", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.ObjectMeta, metav1.ObjectMeta{
					Name:      build.PodName(),
					Namespace: namespace,
					Labels: map[string]string{
						"some/label":     "to-pass-through",
						"kpack.io/build": buildName,
					},
					Annotations: map[string]string{
						"some/annotation": "to-pass-through",
					},
					OwnerReferences: []metav1.OwnerReference{
						*kmeta.NewControllerRef(build),
					},
				})
			})

			it("creates a pod with a correct service account", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, serviceAccount, pod.Spec.ServiceAccountName)
			})

			it("configures the FS Mount Group with the supplied group", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, buildPodBuilderConfig.Gid, *pod.Spec.SecurityContext.FSGroup)
			})

			it("creates init containers with all the build steps", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				var names []string
				for _, container := range pod.Spec.InitContainers {
					names = append(names, container.Name)
				}

				assert.Equal(t, []string{
					"prepare",
					"detect",
					"analyze",
					"restore",
					"build",
					"export",
				}, names)
			})

			it("configures the workspace volume with a subPath", func() {
				build.Spec.Source.SubPath = "some/path"

				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				vol := volumeMountFromContainer(t, pod.Spec.InitContainers, "prepare", "workspace-dir")
				assert.Equal(t, "/workspace", vol.MountPath)
				assert.Equal(t, "", vol.SubPath)

				for _, containerName := range []string{"detect", "analyze", "build", "export"} {
					vol := volumeMountFromContainer(t, pod.Spec.InitContainers, containerName, "workspace-dir")
					assert.Equal(t, "/workspace", vol.MountPath)
					assert.Equal(t, "some/path", vol.SubPath)
				}
			})

			it("configures the bindings", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Contains(t,
					pod.Spec.Volumes,
					corev1.Volume{
						Name: "binding-metadata-database",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "database-configmap",
								},
							},
						},
					},
					corev1.Volume{
						Name: "binding-metadata-apm",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "apm-configmap",
								},
							},
						},
					},
					corev1.Volume{
						Name: "binding-secret-apm",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "apm-secret",
							},
						},
					},
				)

				for _, containerIdx := range []int{1 /* detect */, 4 /* build */} {
					assert.Contains(t,
						pod.Spec.InitContainers[containerIdx].VolumeMounts,
						corev1.VolumeMount{
							Name:      "binding-metadata-database",
							MountPath: "/platform/bindings/database/metadata",
							ReadOnly:  true,
						},
						corev1.VolumeMount{
							Name:      "binding-metadata-apm",
							MountPath: "/platform/bindings/apm/metadata",
							ReadOnly:  true,
						},
						corev1.VolumeMount{
							Name:      "binding-secret-apm",
							MountPath: "/platform/bindings/apm/secret",
							ReadOnly:  true,
						},
					)
				}
			})

			it("configures prepare with docker and git credentials", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
				assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)
				assert.Equal(t, []string{
					directExecute,
					"build-init",
					"-basic-git=git-secret-1=https://github.com",
					"-ssh-git=git-secret-2=https://bitbucket.com",
					"-basic-docker=docker-secret-1=acr.io",
					"-dockerconfig=docker-secret-2",
					"-dockercfg=docker-secret-3",
				}, pod.Spec.InitContainers[0].Args)

				assert.Contains(t,
					pod.Spec.InitContainers[0].VolumeMounts,
					corev1.VolumeMount{
						Name:      "secret-volume-git-secret-1",
						MountPath: "/var/build-secrets/git-secret-1",
					},
					corev1.VolumeMount{
						Name:      "secret-volume-git-secret-2",
						MountPath: "/var/build-secrets/git-secret-2",
					},
					corev1.VolumeMount{
						Name:      "secret-volume-docker-secret-1",
						MountPath: "/var/build-secrets/docker-secret-1",
					},
					corev1.VolumeMount{
						Name:      "secret-volume-docker-secret-2",
						MountPath: "/var/build-secrets/docker-secret-2",
					},
					corev1.VolumeMount{
						Name:      "secret-volume-docker-secret-3",
						MountPath: "/var/build-secrets/docker-secret-3",
					},
				)
			})

			it("configures prepare with the build configuration", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
				assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)
				assert.Equal(t, buildPodBuilderConfig.Uid, *pod.Spec.InitContainers[0].SecurityContext.RunAsUser)
				assert.Equal(t, buildPodBuilderConfig.Gid, *pod.Spec.InitContainers[0].SecurityContext.RunAsGroup)
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "PLATFORM_ENV_VARS",
						Value: `[{"name":"keyA","value":"valueA"},{"name":"keyB","value":"valueB"}]`,
					})
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "IMAGE_TAG",
						Value: "someimage/name",
					})
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "RUN_IMAGE",
						Value: "builderregistry.io/run",
					})
				assert.Subset(t, pod.Spec.InitContainers[0].VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "platform-dir",
						MountPath: "/platform",
					},
					{
						Name:      "workspace-dir",
						MountPath: "/workspace",
					},
					{
						Name:      "home-dir",
						MountPath: "/builder/home",
					},
					{
						Name:      "builder-pull-secrets-dir",
						MountPath: "/builderPullSecrets",
						ReadOnly:  true,
					},
					{
						Name:      "layers-dir",
						MountPath: "/projectMetadata",
					},
				})
			})

			it("configures the prepare step for git source", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, "prepare", pod.Spec.InitContainers[0].Name)
				assert.Equal(t, config.BuildInitImage, pod.Spec.InitContainers[0].Image)
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "GIT_URL",
						Value: build.Spec.Source.Git.URL,
					})
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "GIT_REVISION",
						Value: build.Spec.Source.Git.Revision,
					},
				)
			})

			it("configures prepare with the blob source", func() {
				build.Spec.Source.Git = nil
				build.Spec.Source.Blob = &v1alpha1.Blob{
					URL: "https://some-blobstore.example.com/some-blob",
				}
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, "prepare", pod.Spec.InitContainers[0].Name)
				assert.Equal(t, config.BuildInitImage, pod.Spec.InitContainers[0].Image)
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "BLOB_URL",
						Value: "https://some-blobstore.example.com/some-blob",
					})
			})

			it("configures prepare with the registry source and empty imagePullSecrets when not provided", func() {
				build.Spec.Source.Git = nil
				build.Spec.Source.Blob = nil
				build.Spec.Source.Registry = &v1alpha1.Registry{
					Image: "some-registry.io/some-image",
				}
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, "prepare", pod.Spec.InitContainers[0].Name)
				assert.Contains(t, pod.Spec.InitContainers[0].VolumeMounts,
					corev1.VolumeMount{
						Name:      "image-pull-secrets-dir",
						MountPath: "/imagePullSecrets",
						ReadOnly:  true,
					})
				assert.NotNil(t, *pod.Spec.Volumes[7].EmptyDir)
				assert.Equal(t, config.BuildInitImage, pod.Spec.InitContainers[0].Image)
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "REGISTRY_IMAGE",
						Value: "some-registry.io/some-image",
					})
			})

			it("configures prepare with the registry source and a secret volume when is imagePullSecrets provided", func() {
				build.Spec.Source.Git = nil
				build.Spec.Source.Blob = nil
				build.Spec.Source.Registry = &v1alpha1.Registry{
					Image: "some-registry.io/some-image",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "registry-secret"},
					},
				}
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, "prepare", pod.Spec.InitContainers[0].Name)
				assert.Contains(t, pod.Spec.InitContainers[0].VolumeMounts,
					corev1.VolumeMount{
						Name:      "image-pull-secrets-dir",
						MountPath: "/imagePullSecrets",
						ReadOnly:  true,
					})

				match := 0
				for _, v := range pod.Spec.Volumes {
					if v.Name == "image-pull-secrets-dir" {
						require.NotNil(t, v.Secret)
						assert.Equal(t, "registry-secret", v.Secret.SecretName)
						match++
					}
				}
				assert.Equal(t, 1, match)

				assert.Equal(t, config.BuildInitImage, pod.Spec.InitContainers[0].Image)
				assert.Contains(t, pod.Spec.InitContainers[0].Env,
					corev1.EnvVar{
						Name:  "REGISTRY_IMAGE",
						Value: "some-registry.io/some-image",
					})
			})

			it("configures detect step", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[1].Name, "detect")
				assert.Equal(t, pod.Spec.InitContainers[1].Image, builderImage)
				assert.Equal(t, []string{
					"layers-dir",
					"platform-dir",
					"workspace-dir",
					"binding-metadata-database",
					"binding-metadata-apm",
					"binding-secret-apm",
				}, names(pod.Spec.InitContainers[1].VolumeMounts))
			})

			it("configures analyze step", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[2].Name, "analyze")
				assert.Equal(t, pod.Spec.InitContainers[2].Image, builderImage)
				assert.Equal(t, []string{
					"layers-dir",
					"workspace-dir",
					"home-dir",
					"cache-dir",
				}, names(pod.Spec.InitContainers[2].VolumeMounts))
				assert.Equal(t, []string{
					"-layers=/layers",
					"-group=/layers/group.toml",
					"-analyzed=/layers/analyzed.toml",
					"-cache-dir=/cache",
					build.Spec.LastBuild.Image,
				}, pod.Spec.InitContainers[2].Args)
			})

			it("configures analyze step with the current tag if no previous build", func() {
				build.Spec.LastBuild = nil

				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[2].Name, "analyze")
				assert.Equal(t, pod.Spec.InitContainers[2].Image, builderImage)
				assert.Equal(t, []string{
					"layers-dir",
					"workspace-dir",
					"home-dir",
					"cache-dir",
				}, names(pod.Spec.InitContainers[2].VolumeMounts))
				assert.Equal(t, []string{
					"-layers=/layers",
					"-group=/layers/group.toml",
					"-analyzed=/layers/analyzed.toml",
					"-cache-dir=/cache",
					build.Tag(),
				}, pod.Spec.InitContainers[2].Args)
			})

			it("configures analyze step with the current tag if previous build is corrupted", func() {
				build.Spec.LastBuild = &v1alpha1.LastBuild{}

				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Contains(t, pod.Spec.InitContainers[2].Args, build.Tag())
			})

			it("configures restore step", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[3].Name, "restore")
				assert.Equal(t, pod.Spec.InitContainers[3].Image, builderImage)
				assert.Equal(t, []string{
					"layers-dir",
					"cache-dir",
				}, names(pod.Spec.InitContainers[3].VolumeMounts))

				assert.Equal(t, []string{
					"-group=/layers/group.toml",
					"-layers=/layers",
					"-cache-dir=/cache"},
					pod.Spec.InitContainers[3].Args)
			})

			it("configures build step", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[4].Name, "build")
				assert.Equal(t, pod.Spec.InitContainers[4].Image, builderImage)
				assert.Len(t, pod.Spec.InitContainers[4].VolumeMounts, len([]string{
					"layers-dir",
					"platform-dir",
					"workspace-dir",
					"binding-metadata-database",
					"binding-metadata-apm",
					"binding-secret-apm",
				}))
			})

			it("configures export step", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[5].Name, "export")
				assert.Equal(t, pod.Spec.InitContainers[5].Image, builderImage)
				assert.Equal(t, names(pod.Spec.InitContainers[5].VolumeMounts), []string{
					"layers-dir",
					"workspace-dir",
					"home-dir",
					"cache-dir",
				})
				assert.Equal(t, []string{
					"-layers=/layers",
					"-app=/workspace",
					"-group=/layers/group.toml",
					"-analyzed=/layers/analyzed.toml",
					"-cache-dir=/cache",
					build.Tag(),
					"someimage/name:tag2",
					"someimage/name:tag3",
				}, pod.Spec.InitContainers[5].Args)
			})

			it("configures the builder image in all lifecycle steps", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				for _, container := range pod.Spec.InitContainers {
					if container.Name != "prepare" {
						assert.Equal(t, builderImage, container.Image, fmt.Sprintf("image on container '%s'", container.Name))
					}
				}
			})

			it("configures the completion container with resources", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				completionContainer := pod.Spec.Containers[0]
				assert.Equal(t, resources, completionContainer.Resources)
			})

			it("creates a pod with reusable cache when name is provided", func() {
				pod, err := build.BuildPod(config, nil, buildPodBuilderConfig)
				require.NoError(t, err)

				require.Len(t, pod.Spec.Volumes, 13)
				assert.Equal(t, corev1.Volume{
					Name: "cache-dir",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "some-cache-name"},
					},
				}, pod.Spec.Volumes[0])
			})

			it("creates a pod with empty cache when no name is provided", func() {
				build.Spec.CacheName = ""
				pod, err := build.BuildPod(config, nil, buildPodBuilderConfig)
				require.NoError(t, err)

				require.Len(t, pod.Spec.Volumes, 13)
				assert.Equal(t, corev1.Volume{
					Name: "cache-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				}, pod.Spec.Volumes[0])
			})

			it("attach volumes for secrets", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assertSecretPresent(t, pod, "git-secret-1")
				assertSecretPresent(t, pod, "git-secret-2")
				assertSecretPresent(t, pod, "docker-secret-1")
				assertSecretPresent(t, pod, "docker-secret-2")
				assertSecretPresent(t, pod, "docker-secret-3")
				assertSecretNotPresent(t, pod, "random-secret-1")
			})

			it("attach image pull secrets to pod", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				require.Len(t, pod.Spec.ImagePullSecrets, 1)
				assert.Equal(t, corev1.LocalObjectReference{Name: "some-image-secret"}, pod.Spec.ImagePullSecrets[0])
			})

			it("mounts volumes for bindings", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				require.Len(t, pod.Spec.ImagePullSecrets, 1)
				assert.Equal(t, corev1.LocalObjectReference{Name: "some-image-secret"}, pod.Spec.ImagePullSecrets[0])
			})
		})

		when("0.3 platform api", func() {
			buildPodBuilderConfig.PlatformAPI = "0.3"

			it("calls export with project metadata toml file", func() {
				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.Spec.InitContainers[5].Name, "export")
				assert.Equal(t, pod.Spec.InitContainers[5].Image, builderImage)
				assert.Equal(t, names(pod.Spec.InitContainers[5].VolumeMounts), []string{
					"layers-dir",
					"workspace-dir",
					"home-dir",
					"cache-dir",
					"report-dir",
				})
				assert.Equal(t, []string{
					"-layers=/layers",
					"-app=/workspace",
					"-group=/layers/group.toml",
					"-analyzed=/layers/analyzed.toml",
					"-cache-dir=/cache",
					"-project-metadata=/layers/project-metadata.toml",
					"-report=/var/report/report.toml",
					build.Tag(),
					"someimage/name:tag2",
					"someimage/name:tag3",
				}, pod.Spec.InitContainers[5].Args)
			})
		})

		when("< 0.2 platform api", func() {
			buildPodBuilderConfig.PlatformAPI = "0.1"

			it("returns an error", func() {
				_, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.EqualError(t, err, "incompatible builder platform API version: 0.1")
			})
		})

		when("creating a rebase pod", func() {
			it("creates a pod just to rebase", func() {
				build.Annotations = map[string]string{v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack, "some/annotation": "to-pass-through"}

				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				assert.Equal(t, pod.ObjectMeta, metav1.ObjectMeta{
					Name:      build.PodName(),
					Namespace: namespace,
					Labels: map[string]string{
						"some/label":     "to-pass-through",
						"kpack.io/build": buildName,
					},
					Annotations: map[string]string{
						"some/annotation":              "to-pass-through",
						v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack,
					},
					OwnerReferences: []metav1.OwnerReference{
						*kmeta.NewControllerRef(build),
					},
				})

				require.Equal(t, corev1.PodSpec{
					ServiceAccountName: build.Spec.ServiceAccount,
					Volumes: []corev1.Volume{
						{
							Name: "secret-volume-docker-secret-1",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "docker-secret-1",
								},
							},
						},
						{
							Name: "secret-volume-docker-secret-2",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "docker-secret-2",
								},
							},
						},
						{
							Name: "secret-volume-docker-secret-3",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "docker-secret-3",
								},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "completion",
							Image:           config.CompletionImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources:       build.Spec.Resources,
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  "rebase",
							Image: config.RebaseImage,
							Args: []string{
								directExecute,
								"rebase",
								"--run-image",
								"builderregistry.io/run",
								"--last-built-image",
								build.Spec.LastBuild.Image,
								"-basic-docker=docker-secret-1=acr.io",
								"-dockerconfig=docker-secret-2",
								"-dockercfg=docker-secret-3",
								"someimage/name", "someimage/name:tag2", "someimage/name:tag3"},
							ImagePullPolicy: corev1.PullIfNotPresent,
							WorkingDir:      "/workspace",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "secret-volume-docker-secret-1",
									MountPath: "/var/build-secrets/docker-secret-1",
								},
								{
									Name:      "secret-volume-docker-secret-2",
									MountPath: "/var/build-secrets/docker-secret-2",
								},
								{
									Name:      "secret-volume-docker-secret-3",
									MountPath: "/var/build-secrets/docker-secret-3",
								},
							},
						},
					},
				}, pod.Spec)
			})
		})

		when("a notary config is present on the build", func() {
			it("sets up the completion image to sign the image", func() {
				build.Spec.Notary.V1 = &v1alpha1.NotaryV1Config{
					URL: "some-notary-url",
					SecretRef: v1alpha1.NotarySecretRef{
						Name: "some-notary-secret",
					},
					ConfigMapKeyRef: &v1alpha1.ConfigMapKeyRef{
						Name: "notary-ca-cert",
						Key:  "ca.crt",
					},
				}

				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

				require.Contains(t, pod.Spec.Containers[0].Args, "-notary-v1-url=some-notary-url")
				require.Contains(t, pod.Spec.Containers[0].Args, "-ca-cert=/var/notary/certs/ca.crt")

				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "notary-dir",
					ReadOnly:  true,
					MountPath: "/var/notary/v1",
				})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "report-dir",
					ReadOnly:  false,
					MountPath: "/var/report",
				})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "notary-certs-dir",
					ReadOnly:  true,
					MountPath: "/var/notary/certs",
				})

				require.Contains(t, pod.Spec.Volumes, corev1.Volume{
					Name: "notary-dir",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "some-notary-secret",
						},
					},
				})
				require.Contains(t, pod.Spec.Volumes, corev1.Volume{
					Name: "notary-certs-dir",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "notary-ca-cert",
							},
						},
					},
				})
			})
		})

		it("creates the pod container correctly", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			require.Len(t, pod.Spec.Containers, 1)
			assert.Equal(t, "completion/image:image", pod.Spec.Containers[0].Image)
		})
	})
}

func assertSecretPresent(t *testing.T, pod *corev1.Pod, secretName string) {
	assert.True(t, isSecretPresent(t, pod, secretName), fmt.Sprintf("secret '%s' not present", secretName))
}

func assertSecretNotPresent(t *testing.T, pod *corev1.Pod, secretName string) {
	assert.False(t, isSecretPresent(t, pod, secretName), fmt.Sprintf("secret '%s' not present", secretName))
}

func isSecretPresent(t *testing.T, pod *corev1.Pod, secretName string) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == fmt.Sprintf(v1alpha1.SecretTemplateName, secretName) {
			assert.Equal(t, corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			}, volume.VolumeSource)
			return true
		}
	}
	return false
}

func volumeMountFromContainer(t *testing.T, containers []corev1.Container, containerName string, volumeName string) corev1.VolumeMount {
	t.Helper()
	for _, container := range containers {
		if container.Name == containerName {
			for _, vol := range container.VolumeMounts {
				if vol.Name == volumeName {
					return vol
				}
			}
		}
	}
	t.Errorf("could not find volume mount with name %s in container %s", volumeName, containerName)
	return corev1.VolumeMount{}
}

func names(mounts []corev1.VolumeMount) (names []string) {
	for _, m := range mounts {
		names = append(names, m.Name)
	}
	return
}
