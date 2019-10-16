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
			Env: []corev1.EnvVar{
				{Name: "keyA", Value: "valueA"},
				{Name: "keyB", Value: "valueB"},
			},
			Resources: resources,
			LastBuild: v1alpha1.LastBuild{
				Image: previousAppImage,
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
	}

	config := v1alpha1.BuildPodImages{
		BuildInitImage: "build/init:image",
		NopImage:       "no/op:image",
	}

	buildPodBuilderConfig := v1alpha1.BuildPodBuilderConfig{
		BuilderSpec: builderImageRef,
		StackID:     "com.builder.stack.io",
		RunImage:    "builderregistry.io/run",
		Uid:         2000,
		Gid:         3000,
	}

	when("BuildPod", func() {
		it("creates a pod with a builder owner reference and build label", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			assert.Equal(t, pod.ObjectMeta, metav1.ObjectMeta{
				Name:      build.PodName(),
				Namespace: namespace,
				Labels: map[string]string{
					"some/label":             "to-pass-through",
					"build.pivotal.io/build": buildName,
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

			assert.Len(t, pod.Spec.InitContainers, len([]string{
				"prepare",
				"detect",
				"restore",
				"analyze",
				"build",
				"export",
				"cache",
			}))
		})

		it("configures the workspace volume with a subPath", func() {
			build.Spec.Source.SubPath = "some/path"

			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			vol := getVolumeMountFromContainer(t, pod.Spec.InitContainers, "prepare", "workspace-dir")
			assert.Equal(t, "/workspace", vol.MountPath)
			assert.Equal(t, "", vol.SubPath)

			for _, containerName := range []string{"detect", "analyze", "build", "export"} {
				vol := getVolumeMountFromContainer(t, pod.Spec.InitContainers, containerName, "workspace-dir")
				assert.Equal(t, "/workspace", vol.MountPath)
				assert.Equal(t, "some/path", vol.SubPath)
			}
		})

		it("configures prepare with docker and git credentials", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
			assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)
			assert.Equal(t, []string{
				directExecute,
				"/layers/org.cloudfoundry.go-mod/app-binary/build-init",
				"-basic-git=git-secret-1=https://github.com",
				"-basic-docker=docker-secret-1=acr.io",
			}, pod.Spec.InitContainers[0].Args)

			assert.Contains(t,
				pod.Spec.InitContainers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      "secret-volume-git-secret-1",
					MountPath: "/var/build-secrets/git-secret-1",
				},
				corev1.VolumeMount{
					Name:      "secret-volume-docker-secret-1",
					MountPath: "/var/build-secrets/docker-secret-1",
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
				},
				corev1.EnvVar{
					Name:  "IMAGE_TAG",
					Value: "someimage/name",
				},
				corev1.EnvVar{
					Name:  "HOME",
					Value: "/builder/home",
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
				},
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
			assert.NotNil(t, *pod.Spec.Volumes[5].EmptyDir)
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
					{Name: "foo"},
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
			require.NotNil(t, *pod.Spec.Volumes[5].Secret)
			assert.Equal(t, "foo", pod.Spec.Volumes[5].Secret.SecretName)
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
			assert.Len(t, pod.Spec.InitContainers[1].VolumeMounts, len([]string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
			}))
		})

		it("configures restore step", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[2].Name, "restore")
			assert.Equal(t, pod.Spec.InitContainers[2].Image, builderImage)
			assert.Len(t, pod.Spec.InitContainers[2].VolumeMounts, len([]string{
				"layers-dir",
				"home-dir",
			}))
		})

		it("configures analyze step", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[3].Name, "analyze")
			assert.Equal(t, pod.Spec.InitContainers[3].Image, builderImage)
			assert.Len(t, pod.Spec.InitContainers[3].VolumeMounts, len([]string{
				"layers-dir",
				"workspace-dir",
				"home-dir",
			}))
			assert.Equal(t, []string{
				"-layers=/layers",
				"-helpers=false",
				"-group=/layers/group.toml",
				"-analyzed=/layers/analyzed.toml",
				build.Tag(),
			}, pod.Spec.InitContainers[3].Args)
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
			}))
		})

		it("configures export step", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[5].Name, "export")
			assert.Equal(t, pod.Spec.InitContainers[5].Image, builderImage)
			assert.Len(t, pod.Spec.InitContainers[5].VolumeMounts, len([]string{
				"layers-dir",
				"workspace-dir",
				"home-dir",
			}))
			assert.Equal(t, []string{
				"-layers=/layers",
				"-helpers=false",
				"-app=/workspace",
				"-group=/layers/group.toml",
				"-analyzed=/layers/analyzed.toml",
				build.Tag(),
				"someimage/name:tag2",
				"someimage/name:tag3",
			}, pod.Spec.InitContainers[5].Args)
		})

		it("configures cache step", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[6].Name, "cache")
			assert.Equal(t, pod.Spec.InitContainers[6].Image, builderImage)
			assert.Len(t, pod.Spec.InitContainers[6].VolumeMounts, len([]string{
				"layers-dir",
				"cache-dir",
			}))
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

		it("configures the nop container with resources", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			nopContainer := pod.Spec.Containers[0]
			assert.Equal(t, resources, nopContainer.Resources)
		})

		it("creates a pod with reusable cache when name is provided", func() {
			pod, err := build.BuildPod(config, nil, buildPodBuilderConfig)
			require.NoError(t, err)

			require.Len(t, pod.Spec.Volumes, 7)
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

			require.Len(t, pod.Spec.Volumes, 7)
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
			assertSecretPresent(t, pod, "docker-secret-1")
			assertSecretNotPresent(t, pod, "random-secret-1")
		})

		it("attach image pull secrets to pod", func() {
			pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
			require.NoError(t, err)

			require.Len(t, pod.Spec.ImagePullSecrets, 1)
			assert.Equal(t, corev1.LocalObjectReference{Name: "some-image-secret"}, pod.Spec.ImagePullSecrets[0])
		})

		when("creating a rebase pod", func() {
			it("creates a pod just to rebase", func() {
				build.Annotations = map[string]string{v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack}

				pod, err := build.BuildPod(config, secrets, buildPodBuilderConfig)
				require.NoError(t, err)

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
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "completion",
							Image:           config.NopImage,
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
								"/layers/org.cloudfoundry.go-mod/app-binary/rebase",
								"-basic-docker=docker-secret-1=acr.io",
								"--run-image",
								"builderregistry.io/run",
								"--last-built-image",
								build.Spec.LastBuild.Image,
								"someimage/name", "someimage/name:tag2", "someimage/name:tag3"},
							ImagePullPolicy: corev1.PullIfNotPresent,
							WorkingDir:      "/workspace",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "secret-volume-docker-secret-1",
									MountPath: "/var/build-secrets/docker-secret-1",
								},
							},
						},
					},
				}, pod.Spec)
			})
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

func getVolumeMountFromContainer(t *testing.T, containers []corev1.Container, containerName string, volumeName string) corev1.VolumeMount {
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
