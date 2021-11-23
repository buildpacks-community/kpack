package v1alpha2_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestBuildPod(t *testing.T) {
	spec.Run(t, "Test Build Pod", testBuildPod)
}

func testBuildPod(t *testing.T, when spec.G, it spec.S) {
	const (
		namespace        = "some-namespace"
		buildName        = "build-name"
		builderImage     = "builderregistry.io/builder:latest@sha256:42lkajdsf9q87234"
		previousAppImage = "someimage/name@sha256:previous"
		serviceAccount   = "someserviceaccount"
		dnsProbeHost     = "index.docker.io"
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

	builderImageRef := corev1alpha1.BuildBuilderSpec{
		Image: builderImage,
		ImagePullSecrets: []corev1.LocalObjectReference{
			{Name: "builder-pull-secret"},
		}}

	build := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: namespace,
			Labels: map[string]string{
				"some/label":                 "to-pass-through",
				"image.kpack.io/buildNumber": "12",
			},
			Annotations: map[string]string{
				"some/annotation": "to-pass-through",
			},
			CreationTimestamp: metav1.Date(1944, 6, 6, 13, 30, 0, 0, time.UTC),
		},
		Spec: buildapi.BuildSpec{
			Tags:               []string{"someimage/name", "someimage/name:tag2", "someimage/name:tag3"},
			Builder:            builderImageRef,
			ServiceAccountName: serviceAccount,
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "giturl.com/git.git",
					Revision: "gitrev1234",
				},
			},
			Cache: &buildapi.BuildCacheConfig{
				Volume: &buildapi.BuildPersistentVolumeCache{
					ClaimName: "some-cache-name",
				},
			},
			Services: buildapi.Services{
				{
					Name: "database",
				},
				{
					Name: "apm",
				},
			},
			Env: []corev1.EnvVar{
				{Name: "keyA", Value: "valueA"},
				{Name: "keyB", Value: "valueB"},
			},
			Resources: resources,
			LastBuild: &buildapi.LastBuild{
				Image:   previousAppImage,
				StackId: "com.builder.stack.io",
			},
			Tolerations:  []corev1.Toleration{{Key: "some-key"}},
			NodeSelector: map[string]string{"foo": "bar"},
			Affinity:     &corev1.Affinity{},
		},
	}

	serviceBindings := []buildapi.ServiceBinding{
		&corev1alpha1.ServiceBinding{
			Name: "database",
			SecretRef: &corev1.LocalObjectReference{
				Name: "database",
			},
		},
		&corev1alpha1.ServiceBinding{
			Name: "apm",
			SecretRef: &corev1.LocalObjectReference{
				Name: "apm",
			},
		},
	}

	v1alpha1ServiceBindings := []buildapi.ServiceBinding{
		&corev1alpha1.CNBServiceBinding{
			Name:        "some-v1alpha1-binding",
			SecretRef:   &corev1.LocalObjectReference{Name: "some-secret"},
			MetadataRef: &corev1.LocalObjectReference{Name: "some-configmap"},
		},
	}

	secrets := []corev1.Secret{
		{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "git-secret-1",
				Annotations: map[string]string{
					buildapi.GITSecretAnnotationPrefix: "https://github.com",
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
					buildapi.GITSecretAnnotationPrefix: "https://bitbucket.com",
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
					buildapi.DOCKERSecretAnnotationPrefix: "acr.io",
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
					buildapi.DOCKERSecretAnnotationPrefix: "ignoreme.com",
				},
			},
			Type: corev1.SecretTypeBootstrapToken,
		},
	}

	cosignValidSecrets := []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cosign-secret-1",
				Annotations: map[string]string{
					"kpack.io/cosign.ignored":            "test",
					"kpack.io/cosign.repository":         "testRepository.com/fake-project-1",
					"kpack.io/cosign.docker-media-types": "1",
				},
			},
			Data: map[string][]byte{
				"cosign.key":      []byte("fake-key"),
				"cosign.password": []byte("fake-password"),
			},
			Type: corev1.SecretTypeOpaque,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cosign-secret-no-password-1",
				Annotations: map[string]string{
					"kpack.io/cosign.repository": "testRepository.com/fake-project-2",
				},
			},
			Data: map[string][]byte{
				"cosign.key":      []byte("fake-key"),
				"cosign.password": []byte(""),
			},
			Type: corev1.SecretTypeOpaque,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cosign-secret-no-password-2",
				Annotations: map[string]string{
					"kpack.io/cosign.docker-media-types": "1",
				},
			},
			Data: map[string][]byte{
				"cosign.key": []byte("fake-key"),
			},
			Type: corev1.SecretTypeOpaque,
		},
	}

	cosignInvalidSecrets := []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "invalid-cosign-secret",
			},
			Data: map[string][]byte{
				"cosign.password": []byte("fake-password"),
			},
			Type: corev1.SecretTypeOpaque,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "invalid-empty-cosign-secret",
			},
			Data: map[string][]byte{
				"cosign.key": []byte(""),
			},
			Type: corev1.SecretTypeOpaque,
		},
	}

	config := buildapi.BuildPodImages{
		BuildInitImage:         "build/init:image",
		BuildInitWindowsImage:  "build/init/windows:image",
		CompletionImage:        "completion/image:image",
		CompletionWindowsImage: "completion/image/windows:image",
	}

	buildContext := buildapi.BuildContext{
		BuildPodBuilderConfig: buildapi.BuildPodBuilderConfig{
			StackID:      "com.builder.stack.io",
			RunImage:     "builderregistry.io/run",
			Uid:          2000,
			Gid:          3000,
			PlatformAPIs: []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8"},
			OS:           "linux",
		},
		Secrets:  secrets,
		Bindings: serviceBindings,
		ImagePullSecrets: []corev1.LocalObjectReference{
			{
				Name: "image-pull-1",
			},
			{
				Name: "image-pull-2",
			},
		},
	}

	oldBuildContext := buildapi.BuildContext{
		BuildPodBuilderConfig: buildapi.BuildPodBuilderConfig{
			StackID:      "com.builder.stack.io",
			RunImage:     "builderregistry.io/run",
			Uid:          2000,
			Gid:          3000,
			PlatformAPIs: []string{"0.2", "0.3", "0.4", "0.5", "0.6"},
			OS:           "linux",
		},
		Secrets:  secrets,
		Bindings: serviceBindings,
		ImagePullSecrets: []corev1.LocalObjectReference{
			{
				Name: "image-pull-1",
			},
			{
				Name: "image-pull-2",
			},
		},
	}

	when("BuildPod", func() {
		it("creates a pod with a builder owner reference and build labels and annotations", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.ObjectMeta, metav1.ObjectMeta{
				Name:      build.PodName(),
				Namespace: namespace,
				Labels: map[string]string{
					"some/label":                 "to-pass-through",
					"kpack.io/build":             buildName,
					"image.kpack.io/buildNumber": "12",
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
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, serviceAccount, pod.Spec.ServiceAccountName)
		})

		it("sets the pod tolerations and affinity from the build and merges the os node selector", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, map[string]string{"kubernetes.io/os": "linux", "foo": "bar"}, pod.Spec.NodeSelector)
			assert.Equal(t, build.Spec.Tolerations, pod.Spec.Tolerations)
			assert.Equal(t, build.Spec.Affinity, pod.Spec.Affinity)
		})

		it("handles a nil node selector", func() {
			build.Spec.NodeSelector = nil

			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, map[string]string{"kubernetes.io/os": "linux"}, pod.Spec.NodeSelector)
		})

		it("configures the pod security context to match the builder config user and group", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, buildapi.BuildPodBuilderConfig{
				StackID:      "com.builder.stack.io",
				RunImage:     "builderregistry.io/run",
				Uid:          2000,
				Gid:          3000,
				PlatformAPIs: []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8"},
				OS:           "linux",
			}.Uid, *pod.Spec.SecurityContext.RunAsUser)
			assert.Equal(t, buildapi.BuildPodBuilderConfig{
				StackID:      "com.builder.stack.io",
				RunImage:     "builderregistry.io/run",
				Uid:          2000,
				Gid:          3000,
				PlatformAPIs: []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8"},
				OS:           "linux",
			}.Gid, *pod.Spec.SecurityContext.RunAsGroup)
			assert.Equal(t, buildapi.BuildPodBuilderConfig{
				StackID:      "com.builder.stack.io",
				RunImage:     "builderregistry.io/run",
				Uid:          2000,
				Gid:          3000,
				PlatformAPIs: []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8"},
				OS:           "linux",
			}.Gid, *pod.Spec.SecurityContext.FSGroup)
		})

		it("creates init containers with all the build steps", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			var names []string
			for _, container := range pod.Spec.InitContainers {
				names = append(names, container.Name)
			}

			assert.Equal(t, []string{
				"prepare",
				"analyze",
				"detect",
				"restore",
				"build",
				"export",
			}, names)
		})

		it("creates init containers with all the build steps with older apis", func() {
			pod, err := build.BuildPod(config, oldBuildContext)
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

			pod, err := build.BuildPod(config, buildContext)
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

		it("configures the services", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Contains(t,
				pod.Spec.Volumes,
				corev1.Volume{
					Name: "service-binding-secret-database",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "database",
						},
					},
				},
				corev1.Volume{
					Name: "service-binding-secret-apm",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "apm",
						},
					},
				},
			)

			for _, containerIdx := range []int{2 /* detect */, 4 /* build */} {
				assert.Contains(t,
					pod.Spec.InitContainers[containerIdx].VolumeMounts,
					corev1.VolumeMount{
						Name:      "service-binding-secret-database",
						MountPath: "/platform/bindings/database",
						ReadOnly:  true,
					},
					corev1.VolumeMount{
						Name:      "service-binding-secret-apm",
						MountPath: "/platform/bindings/apm",
						ReadOnly:  true,
					},
				)
			}
		})

		it("configures the services for older apis", func() {
			pod, err := build.BuildPod(config, oldBuildContext)
			require.NoError(t, err)

			assert.Contains(t,
				pod.Spec.Volumes,
				corev1.Volume{
					Name: "service-binding-secret-database",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "database",
						},
					},
				},
				corev1.Volume{
					Name: "service-binding-secret-apm",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "apm",
						},
					},
				},
			)

			for _, containerIdx := range []int{1 /* detect */, 4 /* build */} {
				assert.Contains(t,
					pod.Spec.InitContainers[containerIdx].VolumeMounts,
					corev1.VolumeMount{
						Name:      "service-binding-secret-database",
						MountPath: "/platform/bindings/database",
						ReadOnly:  true,
					},
					corev1.VolumeMount{
						Name:      "service-binding-secret-apm",
						MountPath: "/platform/bindings/apm",
						ReadOnly:  true,
					},
				)
			}
		})

		it("configures the v1alpha1bindings", func() {
			buildContext.Bindings = v1alpha1ServiceBindings
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Contains(t,
				pod.Spec.Volumes,
				corev1.Volume{
					Name: "binding-metadata-some-v1alpha1-binding",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "some-configmap",
							},
						},
					},
				},
				corev1.Volume{
					Name: "binding-secret-some-v1alpha1-binding",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "some-secret",
						},
					},
				},
			)

			for _, containerIdx := range []int{2 /* detect */, 4 /* build */} {
				assert.Contains(t,
					pod.Spec.InitContainers[containerIdx].VolumeMounts,
					corev1.VolumeMount{
						Name:      "binding-metadata-some-v1alpha1-binding",
						MountPath: "/platform/bindings/some-v1alpha1-binding/metadata",
						ReadOnly:  true,
					},
					corev1.VolumeMount{
						Name:      "binding-secret-some-v1alpha1-binding",
						MountPath: "/platform/bindings/some-v1alpha1-binding/secret",
						ReadOnly:  true,
					},
				)
			}
		})

		it("configures the v1alpha1bindings for older apis", func() {
			oldBuildContext.Bindings = v1alpha1ServiceBindings
			pod, err := build.BuildPod(config, oldBuildContext)
			require.NoError(t, err)

			assert.Contains(t,
				pod.Spec.Volumes,
				corev1.Volume{
					Name: "binding-metadata-some-v1alpha1-binding",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "some-configmap",
							},
						},
					},
				},
				corev1.Volume{
					Name: "binding-secret-some-v1alpha1-binding",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "some-secret",
						},
					},
				},
			)

			for _, containerIdx := range []int{1 /* detect */, 4 /* build */} {
				assert.Contains(t,
					pod.Spec.InitContainers[containerIdx].VolumeMounts,
					corev1.VolumeMount{
						Name:      "binding-metadata-some-v1alpha1-binding",
						MountPath: "/platform/bindings/some-v1alpha1-binding/metadata",
						ReadOnly:  true,
					},
					corev1.VolumeMount{
						Name:      "binding-secret-some-v1alpha1-binding",
						MountPath: "/platform/bindings/some-v1alpha1-binding/secret",
						ReadOnly:  true,
					},
				)
			}
		})

		it("configures prepare with docker and git credentials and image pull secrets", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
			assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)
			assert.Equal(t, []string{
				"-basic-git=git-secret-1=https://github.com",
				"-ssh-git=git-secret-2=https://bitbucket.com",
				"-basic-docker=docker-secret-1=acr.io",
				"-dockerconfig=docker-secret-2",
				"-dockercfg=docker-secret-3",
				"-imagepull=image-pull-1",
				"-imagepull=image-pull-2",
				"-imagepull=builder-pull-secret",
			}, pod.Spec.InitContainers[0].Args)

			assert.Subset(t,
				pod.Spec.InitContainers[0].VolumeMounts,
				[]corev1.VolumeMount{
					{
						Name:      "secret-volume-git-secret-1",
						MountPath: "/var/build-secrets/git-secret-1",
					},
					{
						Name:      "secret-volume-git-secret-2",
						MountPath: "/var/build-secrets/git-secret-2",
					},
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

					{
						Name:      "secret-volume-image-pull-1",
						MountPath: "/var/build-secrets/image-pull-1",
					},
					{
						Name:      "secret-volume-image-pull-2",
						MountPath: "/var/build-secrets/image-pull-2",
					},
					{
						Name:      "secret-volume-builder-pull-secret",
						MountPath: "/var/build-secrets/builder-pull-secret",
					}},
			)

		})

		it("configures prepare with the build configuration", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
			assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)
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
					Name:      "layers-dir",
					MountPath: "/projectMetadata",
				},
			})
		})

		it("configures the prepare step for git source", func() {
			pod, err := build.BuildPod(config, buildContext)
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
			build.Spec.Source.Blob = &corev1alpha1.Blob{
				URL: "https://some-blobstore.example.com/some-blob",
			}
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, "prepare", pod.Spec.InitContainers[0].Name)
			assert.Equal(t, config.BuildInitImage, pod.Spec.InitContainers[0].Image)
			assert.Contains(t, pod.Spec.InitContainers[0].Env,
				corev1.EnvVar{
					Name:  "BLOB_URL",
					Value: "https://some-blobstore.example.com/some-blob",
				})
		})

		it("configures prepare with the registry source and a secret volume when is imagePullSecrets provided", func() {
			build.Spec.Source.Git = nil
			build.Spec.Source.Blob = nil
			build.Spec.Source.Registry = &corev1alpha1.Registry{
				Image: "some-registry.io/some-image",
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "registry-secret"},
				},
			}
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, "prepare", pod.Spec.InitContainers[0].Name)
			assert.Contains(t, pod.Spec.InitContainers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      "registry-source-pull-secrets-dir",
					MountPath: "/registrySourcePullSecrets",
					ReadOnly:  true,
				})

			match := 0
			for _, v := range pod.Spec.Volumes {
				if v.Name == "registry-source-pull-secrets-dir" {
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
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[2].Name, "detect")
			assert.Contains(t, pod.Spec.InitContainers[2].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.Equal(t, pod.Spec.InitContainers[2].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
				"service-binding-secret-database",
				"service-binding-secret-apm",
			}, names(pod.Spec.InitContainers[2].VolumeMounts))
		})

		it("configures detect step with the older platform api", func() {
			pod, err := build.BuildPod(config, oldBuildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[1].Name, "detect")
			assert.Contains(t, pod.Spec.InitContainers[1].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.6"})
			assert.Equal(t, pod.Spec.InitContainers[1].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
				"service-binding-secret-database",
				"service-binding-secret-apm",
			}, names(pod.Spec.InitContainers[1].VolumeMounts))
		})

		it("configures detect step with cnb bindings", func() {
			buildContext.Bindings = v1alpha1ServiceBindings
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[2].Name, "detect")
			assert.Contains(t, pod.Spec.InitContainers[2].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.Equal(t, pod.Spec.InitContainers[2].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
				"binding-metadata-some-v1alpha1-binding",
				"binding-secret-some-v1alpha1-binding",
			}, names(pod.Spec.InitContainers[2].VolumeMounts))
		})

		it("configures analyze step", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[1].Name, "analyze")
			assert.Contains(t, pod.Spec.InitContainers[1].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.Equal(t, pod.Spec.InitContainers[1].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"workspace-dir",
				"home-dir",
			}, names(pod.Spec.InitContainers[1].VolumeMounts))
			tags := []string{}
			for _, tag := range build.Spec.Tags[1:] {
				tags = append(tags, "-tag="+tag)
			}
			assert.Equal(t, append(append([]string{
				"-layers=/layers",
				"-analyzed=/layers/analyzed.toml"},
				tags...),
				"-previous-image="+build.Spec.LastBuild.Image, build.Tag()), pod.Spec.InitContainers[1].Args)
		})

		it("configures analyze step with the older api", func() {
			pod, err := build.BuildPod(config, oldBuildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[2].Name, "analyze")
			assert.Contains(t, pod.Spec.InitContainers[2].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.6"})
			assert.Equal(t, pod.Spec.InitContainers[2].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"workspace-dir",
				"home-dir",
				"cache-dir",
			}, names(pod.Spec.InitContainers[2].VolumeMounts))
			assert.Equal(t, []string{
				"-layers=/layers",
				"-analyzed=/layers/analyzed.toml",
				"-cache-dir=/cache",
				"-group=/layers/group.toml",
				build.Spec.LastBuild.Image,
			}, pod.Spec.InitContainers[2].Args)
		})

		it("configures analyze step with the current tag if no previous build", func() {
			build.Spec.LastBuild = nil

			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[1].Name, "analyze")
			assert.Equal(t, pod.Spec.InitContainers[1].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"workspace-dir",
				"home-dir",
			}, names(pod.Spec.InitContainers[1].VolumeMounts))
			tags := []string{}
			for _, tag := range build.Spec.Tags[1:] {
				tags = append(tags, "-tag="+tag)
			}
			assert.Equal(t, append(append([]string{
				"-layers=/layers",
				"-analyzed=/layers/analyzed.toml"},
				tags...),
				build.Tag()), pod.Spec.InitContainers[1].Args)
		})

		it("configures analyze step with the current tag if no previous build for the older api", func() {
			build.Spec.LastBuild = nil

			pod, err := build.BuildPod(config, oldBuildContext)
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
				"-analyzed=/layers/analyzed.toml",
				"-cache-dir=/cache",
				"-group=/layers/group.toml",
				build.Tag()}, pod.Spec.InitContainers[2].Args)
		})

		it("configures analyze step with the current tag if previous build is corrupted", func() {
			build.Spec.LastBuild = &buildapi.LastBuild{}

			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Contains(t, pod.Spec.InitContainers[1].Args, build.Tag())
		})

		it("configures restore step", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[3].Name, "restore")
			assert.Contains(t, pod.Spec.InitContainers[3].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.Contains(t, pod.Spec.InitContainers[3].Env, corev1.EnvVar{Name: "HOME", Value: "/builder/home"})
			assert.Equal(t, pod.Spec.InitContainers[3].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"home-dir",
				"cache-dir",
			}, names(pod.Spec.InitContainers[3].VolumeMounts))

			assert.Equal(t, []string{
				"-group=/layers/group.toml",
				"-layers=/layers",
				"-cache-dir=/cache",
				"-analyzed=/layers/analyzed.toml"},
				pod.Spec.InitContainers[3].Args)
		})

		it("configures restore step with the older api", func() {
			pod, err := build.BuildPod(config, oldBuildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[3].Name, "restore")
			assert.Contains(t, pod.Spec.InitContainers[3].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.6"})
			assert.Contains(t, pod.Spec.InitContainers[3].Env, corev1.EnvVar{Name: "HOME", Value: "/builder/home"})
			assert.Equal(t, pod.Spec.InitContainers[3].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"home-dir",
				"cache-dir",
			}, names(pod.Spec.InitContainers[3].VolumeMounts))

			assert.Equal(t, []string{
				"-group=/layers/group.toml",
				"-layers=/layers",
				"-cache-dir=/cache"},
				pod.Spec.InitContainers[3].Args)
		})

		it("configures build step", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[4].Name, "build")
			assert.Contains(t, pod.Spec.InitContainers[4].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.Equal(t, pod.Spec.InitContainers[4].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
				"service-binding-secret-database",
				"service-binding-secret-apm",
			}, names(pod.Spec.InitContainers[4].VolumeMounts))
		})

		it("configures the build step with v1alpha1bindings", func() {
			buildContext.Bindings = v1alpha1ServiceBindings
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[4].Name, "build")
			assert.Contains(t, pod.Spec.InitContainers[4].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.Equal(t, pod.Spec.InitContainers[4].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
				"binding-metadata-some-v1alpha1-binding",
				"binding-secret-some-v1alpha1-binding",
			}, names(pod.Spec.InitContainers[4].VolumeMounts))
		})

		it("configures export step", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[5].Name, "export")
			assert.Equal(t, pod.Spec.InitContainers[5].Image, builderImage)
			assert.Contains(t, pod.Spec.InitContainers[5].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.ElementsMatch(t, names(pod.Spec.InitContainers[5].VolumeMounts), []string{
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
				"-project-metadata=/layers/project-metadata.toml",
				"-cache-dir=/cache",
				"-report=/var/report/report.toml",
				build.Tag(),
				"someimage/name:tag2",
				"someimage/name:tag3",
			}, pod.Spec.InitContainers[5].Args)
		})
		it("configures export step with non-web default process", func() {
			build.Spec.DefaultProcess = "sys-info"
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[5].Name, "export")
			assert.Equal(t, pod.Spec.InitContainers[5].Image, builderImage)
			assert.Contains(t, pod.Spec.InitContainers[5].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"})
			assert.ElementsMatch(t, names(pod.Spec.InitContainers[5].VolumeMounts), []string{
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
				"-project-metadata=/layers/project-metadata.toml",
				"-cache-dir=/cache",
				"-process-type=sys-info",
				"-report=/var/report/report.toml",
				build.Tag(),
				"someimage/name:tag2",
				"someimage/name:tag3",
			}, pod.Spec.InitContainers[5].Args)
		})

		it("configures the builder image in all lifecycle steps", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			for _, container := range pod.Spec.InitContainers {
				if container.Name != "prepare" {
					assert.Equal(t, builderImage, container.Image, fmt.Sprintf("image on container '%s'", container.Name))
				}
			}
		})

		it("configures the init containers with resources", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			initContainers := pod.Spec.InitContainers
			for _, i := range initContainers {
				assert.Equal(t, resources, i.Resources)
			}
		})

		it("configures the completion container with resources", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			completionContainer := pod.Spec.Containers[0]
			assert.Equal(t, resources, completionContainer.Resources)
		})

		it("creates a pod with reusable cache when name is provided", func() {
			buildContext.Secrets = nil
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Contains(t, pod.Spec.Volumes, corev1.Volume{
				Name: "cache-dir",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "some-cache-name"},
				},
			})
		})

		when("registry cache is requested (first build)", func() {
			podWithVolumeCache, _ := build.BuildPod(config, buildContext)
			build.Spec.Cache.Volume = nil
			build.Spec.Cache.Registry = &buildapi.RegistryCache{Tag: "test-cache-image"}

			it("creates a pod without cache volume", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Len(t, podWithImageCache.Spec.Volumes, len(podWithVolumeCache.Spec.Volumes)-1)
			})

			it("does not add the cache to analyze container", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				analyzeContainer := podWithImageCache.Spec.InitContainers[2]
				assert.NotContains(t, analyzeContainer.Args, "-cache-image=test-cache-image")
			})
			it("does not add the cache to restore container", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				restoreContainer := podWithImageCache.Spec.InitContainers[3]
				assert.NotContains(t, restoreContainer.Args, "-cache-image=test-cache-image")
			})
			it("adds the cache to export container", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				exportContainer := podWithImageCache.Spec.InitContainers[5]
				assert.Contains(t, exportContainer.Args, "-cache-image=test-cache-image")
			})
		})

		when("registry cache is requested (second build)", func() {
			podWithVolumeCache, _ := build.BuildPod(config, buildContext)
			build.Spec.Cache.Volume = nil
			build.Spec.Cache.Registry = &buildapi.RegistryCache{Tag: "test-cache-image"}
			build.Spec.LastBuild = &buildapi.LastBuild{
				Cache: buildapi.BuildCache{
					Image: "test-cache-image@sha",
				},
			}

			it("creates a pod without cache volume", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Len(t, podWithImageCache.Spec.Volumes, len(podWithVolumeCache.Spec.Volumes)-1)
			})

			it("adds the cache to analyze container", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				analyzeContainer := podWithImageCache.Spec.InitContainers[1]
				assert.Contains(t, analyzeContainer.Args, "-cache-image=test-cache-image@sha")
			})
			it("adds the cache to restore container", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				restoreContainer := podWithImageCache.Spec.InitContainers[3]
				assert.Contains(t, restoreContainer.Args, "-cache-image=test-cache-image@sha")
			})
			it("adds the cache to export container", func() {
				podWithImageCache, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				exportContainer := podWithImageCache.Spec.InitContainers[5]
				assert.Contains(t, exportContainer.Args, "-cache-image=test-cache-image")
			})
		})

		when("ImageTag is empty", func() {
			var pod *corev1.Pod
			var err error
			build.Spec.Cache.Registry = &buildapi.RegistryCache{Tag: ""}

			it("does not add the cache to analyze container", func() {
				pod, err = build.BuildPod(config, buildContext)
				require.NoError(t, err)

				analyzeContainer := pod.Spec.InitContainers[2]
				assert.NotContains(t, analyzeContainer.Args, "-cache-image")
			})
			it("does not add the cache to restore container", func() {
				pod, err = build.BuildPod(config, buildContext)
				require.NoError(t, err)

				restoreContainer := pod.Spec.InitContainers[3]
				assert.NotContains(t, restoreContainer.Args, "-cache-image")
			})
			it("does not add the cache to export container", func() {
				pod, err = build.BuildPod(config, buildContext)
				require.NoError(t, err)

				exportContainer := pod.Spec.InitContainers[5]
				assert.NotContains(t, exportContainer.Args, "-cache-image")
			})
		})

		when("Cache is nil", func() {
			buildCopy := build.DeepCopy()
			podWithCache, _ := buildCopy.BuildPod(config, buildContext)
			buildCopy.Spec.Cache = nil

			it("creates a pod without cache volume", func() {
				pod, err := buildCopy.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Len(t, pod.Spec.Volumes, len(podWithCache.Spec.Volumes)-1)
			})
		})

		when("CacheName is empty", func() {
			podWithCache, _ := build.BuildPod(config, buildContext)
			build.Spec.Cache.Volume = &buildapi.BuildPersistentVolumeCache{ClaimName: ""}

			it("creates a pod without cache volume", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Len(t, pod.Spec.Volumes, len(podWithCache.Spec.Volumes)-1)
			})

			it("does not add the cache to analyze container", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				analyzeContainer := pod.Spec.InitContainers[1]
				assert.Equal(t, analyzeContainer.Name, "analyze")
				assert.NotContains(t, analyzeContainer.Args, "-cache-dir=/cache")

			})

			it("does not add the cache to restore container", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				restoreContainer := pod.Spec.InitContainers[3]
				assert.NotContains(t, restoreContainer.Args, "-cache-dir=/cache")
				assert.Len(t, restoreContainer.VolumeMounts, len(podWithCache.Spec.InitContainers[3].VolumeMounts)-1)
			})

			it("does not add the cache to exporter container", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				exportContainer := pod.Spec.InitContainers[5]
				assert.NotContains(t, exportContainer.Args, "-cache-dir=/cache")
				assert.Len(t, exportContainer.VolumeMounts, len(podWithCache.Spec.InitContainers[5].VolumeMounts)-1)
			})
		})

		it("attach volumes for secrets", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assertSecretPresent(t, pod, "git-secret-1")
			assertSecretPresent(t, pod, "git-secret-2")
			assertSecretPresent(t, pod, "docker-secret-1")
			assertSecretPresent(t, pod, "docker-secret-2")
			assertSecretPresent(t, pod, "docker-secret-3")
			assertSecretPresent(t, pod, "image-pull-1")
			assertSecretPresent(t, pod, "image-pull-2")
			assertSecretPresent(t, pod, "builder-pull-secret")
			assertSecretNotPresent(t, pod, "random-secret-1")
		})

		it("deduplicates builder imagepullSecrets from service account image pull secrets", func() {
			buildContext.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "duplicated-secret"}}
			build.Spec.Builder.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "duplicated-secret"}}

			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			volumeNames := map[string]struct{}{}
			for _, v := range pod.Spec.Volumes {
				volumeNames[v.Name] = struct{}{}
			}

			require.Len(t, pod.Spec.Volumes, len(volumeNames))
		})

		it("attach image pull secrets to pod", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			require.Len(t, pod.Spec.ImagePullSecrets, 1)
			assert.Equal(t, corev1.LocalObjectReference{Name: "builder-pull-secret"}, pod.Spec.ImagePullSecrets[0])
		})

		when("only 0.3 platform api is supported", func() {
			buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

			it("exports without a report and without default process type", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Contains(t, pod.Spec.InitContainers[5].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.3"})
				assert.Equal(t, []string{
					"-layers=/layers",
					"-app=/workspace",
					"-group=/layers/group.toml",
					"-analyzed=/layers/analyzed.toml",
					"-project-metadata=/layers/project-metadata.toml",
					"-cache-dir=/cache",
					build.Tag(),
					"someimage/name:tag2",
					"someimage/name:tag3",
				}, pod.Spec.InitContainers[5].Args)
			})
		})

		when("no supported platform apis are available", func() {
			buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.2", "0.999"}

			it("returns an error", func() {
				_, err := build.BuildPod(config, buildContext)
				require.EqualError(t, err, "unsupported builder platform API versions: 0.2,0.999")
			})
		})

		when("creating a rebase pod", func() {
			build.Annotations = map[string]string{
				buildapi.BuildReasonAnnotation:  buildapi.BuildReasonStack,
				buildapi.BuildChangesAnnotation: "some-stack-change",
				"some/annotation":               "to-pass-through",
			}

			it("creates a pod just to rebase", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Equal(t, pod.ObjectMeta, metav1.ObjectMeta{
					Name:      build.PodName(),
					Namespace: namespace,
					Labels: map[string]string{
						"some/label":                 "to-pass-through",
						"kpack.io/build":             buildName,
						"image.kpack.io/buildNumber": "12",
					},
					Annotations: map[string]string{
						"some/annotation":               "to-pass-through",
						buildapi.BuildReasonAnnotation:  buildapi.BuildReasonStack,
						buildapi.BuildChangesAnnotation: "some-stack-change",
					},
					OwnerReferences: []metav1.OwnerReference{
						*kmeta.NewControllerRef(build),
					},
				})

				require.Equal(t, build.Spec.ServiceAccountName, pod.Spec.ServiceAccountName)
				require.Equal(t, build.Spec.Tolerations, pod.Spec.Tolerations)
				require.Equal(t, build.Spec.Affinity, pod.Spec.Affinity)
				require.Equal(t, build.Spec.NodeSelector, map[string]string{
					"kubernetes.io/os": "linux",
					"foo":              "bar",
				})
				require.Equal(t, pod.Spec.Volumes, []corev1.Volume{
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
					{
						Name: "secret-volume-image-pull-1",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "image-pull-1",
							},
						},
					},
					{
						Name: "secret-volume-image-pull-2",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "image-pull-2",
							},
						},
					},
					{
						Name: "secret-volume-builder-pull-secret",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "builder-pull-secret",
							},
						},
					},
					{
						Name: "report-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
					{
						Name: "notary-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})

				require.Equal(t, []corev1.Container{
					{
						Name:      "rebase",
						Image:     config.RebaseImage,
						Resources: build.Spec.Resources,
						Args: []string{
							"--run-image",
							"builderregistry.io/run",
							"--last-built-image",
							build.Spec.LastBuild.Image,
							"--report",
							"/var/report/report.toml",
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
							"-imagepull=image-pull-1",
							"-imagepull=image-pull-2",
							"-imagepull=builder-pull-secret",
							"someimage/name", "someimage/name:tag2", "someimage/name:tag3",
						},
						Env: []corev1.EnvVar{
							{
								Name:  "BUILD_CHANGES",
								Value: "some-stack-change",
							},
						},
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
							{
								Name:      "secret-volume-image-pull-1",
								MountPath: "/var/build-secrets/image-pull-1",
							},
							{
								Name:      "secret-volume-image-pull-2",
								MountPath: "/var/build-secrets/image-pull-2",
							},
							{
								Name:      "secret-volume-builder-pull-secret",
								MountPath: "/var/build-secrets/builder-pull-secret",
							},
							{
								Name:      "report-dir",
								MountPath: "/var/report",
							},
						},
					},
				}, pod.Spec.InitContainers)
			})

			when("cosign secrets are present on the build", func() {
				it("skips invalid secrets", func() {
					buildContext.Secrets = append(secrets, cosignInvalidSecrets...)
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					assert.NotNil(t, pod.Spec.Containers[0])
					assert.NotNil(t, pod.Spec.Containers[0].Command[0])
					assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

					invalidSecretName := "invalid-cosign-secret"
					assertSecretNotPresent(t, pod, invalidSecretName)

					require.NotContains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      fmt.Sprintf("secret-volume-%s", invalidSecretName),
						MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", invalidSecretName),
					})
				})

				it("sets up the completion image to use cosign secrets", func() {
					buildContext.Secrets = append(secrets, cosignValidSecrets...)
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					validSecrets := []string{
						"cosign-secret-1",
						"cosign-secret-no-password-1",
						"cosign-secret-no-password-2",
					}

					for _, secretName := range validSecrets {
						assertSecretPresent(t, pod, secretName)

						require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
							Name:      fmt.Sprintf("secret-volume-%s", secretName),
							MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", secretName),
						})
					}

					require.Equal(t,
						[]string{
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						},
						pod.Spec.Containers[0].Args,
					)
				})

				it("handles custom cosign annotations", func() {
					build.Spec.Cosign = &buildapi.CosignConfig{
						Annotations: []buildapi.CosignAnnotation{
							{Name: "customAnnotationKey", Value: "customAnnotationValue"},
						},
					}
					buildContext.Secrets = append(secrets, cosignValidSecrets...)
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					require.Equal(t,
						[]string{
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-annotations=customAnnotationKey=customAnnotationValue",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						},
						pod.Spec.Containers[0].Args,
					)
				})

				when("a notary config is present on the build", func() {
					it("sets up the completion image to sign the image", func() {
						build.Spec.Notary = &corev1alpha1.NotaryConfig{
							V1: &corev1alpha1.NotaryV1Config{
								URL: "some-notary-url",
								SecretRef: corev1alpha1.NotarySecretRef{
									Name: "some-notary-secret",
								},
							},
						}

						pod, err := build.BuildPod(config, buildContext)
						require.NoError(t, err)
						require.Equal(t,
							[]string{
								"-notary-v1-url=some-notary-url",
								"-basic-docker=docker-secret-1=acr.io",
								"-dockerconfig=docker-secret-2",
								"-dockercfg=docker-secret-3",
								"-cosign-annotations=buildTimestamp=19440606.133000",
								"-cosign-annotations=buildNumber=12",
							},
							pod.Spec.Containers[0].Args,
						)

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

						require.Contains(t, pod.Spec.Volumes, corev1.Volume{
							Name: "notary-dir",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "some-notary-secret",
								},
							},
						})
					})
				})
			})

			when("a notary config is present on the build", func() {
				build.Spec.Notary = &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "some-notary-url",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "some-notary-secret",
						},
					},
				}

				it("errs if platformApi does not support report.toml", func() {
					buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

					_, err := build.BuildPod(config, buildContext)
					require.EqualError(t, err, "unsupported builder platform API versions: 0.3,0.2")
				})

				it("sets up the completion image to sign the image", func() {
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

					require.Subset(t,
						pod.Spec.Containers[0].Args,
						[]string{
							"-notary-v1-url=some-notary-url",
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
						},
					)

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

					require.Contains(t, pod.Spec.Volumes, corev1.Volume{
						Name: "notary-dir",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "some-notary-secret",
							},
						},
					})
				})
			})

			when("cosign secrets and a notary config are present on the build", func() {
				build.Spec.Notary = &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "some-notary-url",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "some-notary-secret",
						},
					},
				}

				it("skips invalid secrets", func() {
					buildContext.Secrets = append(secrets, cosignInvalidSecrets...)
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

					invalidSecretName := "invalid-cosign-secret"
					assertSecretNotPresent(t, pod, invalidSecretName)

					require.NotContains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      fmt.Sprintf("secret-volume-%s", invalidSecretName),
						MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", invalidSecretName),
					})
				})

				it("sets up the completion image to use cosign secrets", func() {
					buildContext.Secrets = append(secrets, cosignValidSecrets...)
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

					validSecrets := []string{
						"cosign-secret-1",
						"cosign-secret-no-password-1",
						"cosign-secret-no-password-2",
					}

					for _, secretName := range validSecrets {
						assertSecretPresent(t, pod, secretName)

						require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
							Name:      fmt.Sprintf("secret-volume-%s", secretName),
							MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", secretName),
						})
					}

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

					require.Contains(t, pod.Spec.Volumes, corev1.Volume{
						Name: "notary-dir",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "some-notary-secret",
							},
						},
					})

					require.Equal(t,
						[]string{
							"-notary-v1-url=some-notary-url",
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						},
						pod.Spec.Containers[0].Args,
					)
				})

				it("handles custom cosign annotations", func() {
					build.Spec.Cosign = &buildapi.CosignConfig{
						Annotations: []buildapi.CosignAnnotation{
							{Name: "customAnnotationKey", Value: "customAnnotationValue"},
						},
					}
					buildContext.Secrets = append(secrets, cosignValidSecrets...)
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					require.Equal(t,
						[]string{
							"-notary-v1-url=some-notary-url",
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-annotations=customAnnotationKey=customAnnotationValue",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						},
						pod.Spec.Containers[0].Args,
					)
				})

				it("errs if platformApi does not support report.toml", func() {
					buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

					_, err := build.BuildPod(config, buildContext)
					require.EqualError(t, err, "unsupported builder platform API versions: 0.3,0.2")
				})
			})
		})

		when("cosign secrets are present on the build", func() {
			it("skips invalid secrets", func() {
				buildContext.Secrets = append(secrets, cosignInvalidSecrets...)
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.NotNil(t, pod.Spec.Containers[0])
				assert.NotNil(t, pod.Spec.Containers[0].Command[0])
				assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

				invalidSecretName := "invalid-cosign-secret"
				assertSecretNotPresent(t, pod, invalidSecretName)

				require.NotContains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf("secret-volume-%s", invalidSecretName),
					MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", invalidSecretName),
				})
			})

			it("sets up the completion image to use cosign secrets", func() {
				buildContext.Secrets = append(secrets, cosignValidSecrets...)
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				validSecrets := []string{
					"cosign-secret-1",
					"cosign-secret-no-password-1",
					"cosign-secret-no-password-2",
				}

				for _, secretName := range validSecrets {
					assertSecretPresent(t, pod, secretName)

					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      fmt.Sprintf("secret-volume-%s", secretName),
						MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", secretName),
					})
				}

				require.Equal(t,
					[]string{
						"-basic-git=git-secret-1=https://github.com",
						"-ssh-git=git-secret-2=https://bitbucket.com",
						"-basic-docker=docker-secret-1=acr.io",
						"-dockerconfig=docker-secret-2",
						"-dockercfg=docker-secret-3",
						"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
						"-cosign-docker-media-types=cosign-secret-1=1",
						"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
						"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						"-cosign-annotations=buildTimestamp=19440606.133000",
						"-cosign-annotations=buildNumber=12",
					},
					pod.Spec.Containers[0].Args,
				)
			})

			it("handles custom cosign annotations", func() {
				build.Spec.Cosign = &buildapi.CosignConfig{
					Annotations: []buildapi.CosignAnnotation{
						{Name: "customAnnotationKey", Value: "customAnnotationValue"},
					},
				}
				buildContext.Secrets = append(secrets, cosignValidSecrets...)
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				require.Equal(t,
					[]string{
						"-basic-git=git-secret-1=https://github.com",
						"-ssh-git=git-secret-2=https://bitbucket.com",
						"-basic-docker=docker-secret-1=acr.io",
						"-dockerconfig=docker-secret-2",
						"-dockercfg=docker-secret-3",
						"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
						"-cosign-docker-media-types=cosign-secret-1=1",
						"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
						"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						"-cosign-annotations=buildTimestamp=19440606.133000",
						"-cosign-annotations=buildNumber=12",
						"-cosign-annotations=customAnnotationKey=customAnnotationValue",
					},
					pod.Spec.Containers[0].Args,
				)
			})
		})

		when("a notary config is present on the build", func() {
			build.Spec.Notary = &corev1alpha1.NotaryConfig{
				V1: &corev1alpha1.NotaryV1Config{
					URL: "some-notary-url",
					SecretRef: corev1alpha1.NotarySecretRef{
						Name: "some-notary-secret",
					},
				},
			}

			it("errs if platformApi does not support report.toml", func() {
				buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

				_, err := build.BuildPod(config, buildContext)
				require.EqualError(t, err, "unsupported builder platform API versions: 0.3,0.2")
			})

			it("sets up the completion image to sign the image", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

				require.Subset(t,
					pod.Spec.Containers[0].Args,
					[]string{
						"-notary-v1-url=some-notary-url",
						"-basic-docker=docker-secret-1=acr.io",
						"-dockerconfig=docker-secret-2",
						"-dockercfg=docker-secret-3",
					},
				)

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

				require.Contains(t, pod.Spec.Volumes, corev1.Volume{
					Name: "notary-dir",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "some-notary-secret",
						},
					},
				})
				require.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{Name: "HOME", Value: "/builder/home"})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "home-dir",
					ReadOnly:  false,
					MountPath: "/builder/home",
				})
			})
		})

		when("cosign secrets and a notary config are present on the build", func() {
			it.Before(func() {
				build.Spec.Notary = &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "some-notary-url",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "some-notary-secret",
						},
					},
				}
			})

			it("skips invalid secrets", func() {
				buildContext.Secrets = append(secrets, cosignInvalidSecrets...)
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

				invalidSecretName := "invalid-cosign-secret"
				assertSecretNotPresent(t, pod, invalidSecretName)

				require.NotContains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      fmt.Sprintf("secret-volume-%s", invalidSecretName),
					MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", invalidSecretName),
				})
			})

			it("sets up the completion image to use cosign secrets", func() {
				buildContext.Secrets = append(secrets, cosignValidSecrets...)
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Equal(t, "/cnb/process/web", pod.Spec.Containers[0].Command[0])

				validSecrets := []string{
					"cosign-secret-1",
					"cosign-secret-no-password-1",
					"cosign-secret-no-password-2",
				}

				for _, secretName := range validSecrets {
					assertSecretPresent(t, pod, secretName)

					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      fmt.Sprintf("secret-volume-%s", secretName),
						MountPath: fmt.Sprintf("/var/build-secrets/cosign/%s", secretName),
					})
				}

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

				require.Contains(t, pod.Spec.Volumes, corev1.Volume{
					Name: "notary-dir",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "some-notary-secret",
						},
					},
				})

				require.Contains(t, pod.Spec.Containers[0].Env, corev1.EnvVar{Name: "HOME", Value: "/builder/home"})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "home-dir",
					ReadOnly:  false,
					MountPath: "/builder/home",
				})

				require.Equal(t,
					[]string{
						"-notary-v1-url=some-notary-url",
						"-basic-git=git-secret-1=https://github.com",
						"-ssh-git=git-secret-2=https://bitbucket.com",
						"-basic-docker=docker-secret-1=acr.io",
						"-dockerconfig=docker-secret-2",
						"-dockercfg=docker-secret-3",
						"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
						"-cosign-docker-media-types=cosign-secret-1=1",
						"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
						"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						"-cosign-annotations=buildTimestamp=19440606.133000",
						"-cosign-annotations=buildNumber=12",
					},
					pod.Spec.Containers[0].Args,
				)
			})

			it("handles custom cosign annotations", func() {
				build.Spec.Cosign = &buildapi.CosignConfig{
					Annotations: []buildapi.CosignAnnotation{
						{Name: "customAnnotationKey", Value: "customAnnotationValue"},
					},
				}
				buildContext.Secrets = append(secrets, cosignValidSecrets...)
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				require.Equal(t,
					[]string{
						"-notary-v1-url=some-notary-url",
						"-basic-git=git-secret-1=https://github.com",
						"-ssh-git=git-secret-2=https://bitbucket.com",
						"-basic-docker=docker-secret-1=acr.io",
						"-dockerconfig=docker-secret-2",
						"-dockercfg=docker-secret-3",
						"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
						"-cosign-docker-media-types=cosign-secret-1=1",
						"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
						"-cosign-docker-media-types=cosign-secret-no-password-2=1",
						"-cosign-annotations=buildTimestamp=19440606.133000",
						"-cosign-annotations=buildNumber=12",
						"-cosign-annotations=customAnnotationKey=customAnnotationValue",
					},
					pod.Spec.Containers[0].Args,
				)
			})

			it("errs if platformApi does not support report.toml", func() {
				buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

				_, err := build.BuildPod(config, buildContext)
				require.EqualError(t, err, "unsupported builder platform API versions: 0.3,0.2")
			})
		})

		it("creates the pod container correctly", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			require.Len(t, pod.Spec.Containers, 1)
			assert.Equal(t, "completion/image:image", pod.Spec.Containers[0].Image)
		})

		when("builder is windows", func() {
			buildContext.BuildPodBuilderConfig.OS = "windows"
			oldBuildContext.BuildPodBuilderConfig.OS = "windows"

			it("errs if platformApi does not support windows", func() {
				buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

				_, err := build.BuildPod(config, buildContext)
				require.EqualError(t, err, "unsupported builder platform API versions: 0.3,0.2")
			})

			it("uses windows node selector", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Equal(t, map[string]string{"kubernetes.io/os": "windows", "foo": "bar"}, pod.Spec.NodeSelector)
			})

			it("removes the spec securityContext", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Nil(t, pod.Spec.SecurityContext)
			})

			it("configures prepare for windows build init", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				prepareContainer := pod.Spec.InitContainers[0]
				assert.Equal(t, "prepare", prepareContainer.Name)
				assert.Equal(t, config.BuildInitWindowsImage, prepareContainer.Image)
				assert.Equal(t, []string{
					"-basic-git=git-secret-1=https://github.com",
					"-ssh-git=git-secret-2=https://bitbucket.com",
					"-basic-docker=docker-secret-1=acr.io",
					"-dockerconfig=docker-secret-2",
					"-dockercfg=docker-secret-3",
					"-imagepull=image-pull-1",
					"-imagepull=image-pull-2",
					"-imagepull=builder-pull-secret",
				}, prepareContainer.Args)

				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, prepareContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})

				assert.Nil(t, prepareContainer.SecurityContext)
			})

			it("configures detect step for windows", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				detectContainer := pod.Spec.InitContainers[2]
				assert.Equal(t, "detect", detectContainer.Name)
				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, detectContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})
				assert.Equal(t, []string{"/networkWait/network-wait-launcher"}, detectContainer.Command)
				assert.Equal(t, []string{
					dnsProbeHost,
					"--",
					"/cnb/lifecycle/detector",
					"-app=/workspace",
					"-group=/layers/group.toml",
					"-plan=/layers/plan.toml",
				}, detectContainer.Args)
			})

			it("configures analyze step", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				analyzeContainer := pod.Spec.InitContainers[1]
				assert.Equal(t, "analyze", analyzeContainer.Name)
				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, analyzeContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})
				assert.Subset(t, analyzeContainer.Env, []corev1.EnvVar{
					{
						Name:  "USERPROFILE",
						Value: "/builder/home",
					},
				})
				assert.Equal(t, []string{"/networkWait/network-wait-launcher"}, analyzeContainer.Command)
				tags := []string{}
				for _, tag := range build.Spec.Tags[1:] {
					tags = append(tags, "-tag="+tag)
				}
				assert.Equal(t, append(append([]string{
					dnsProbeHost,
					"--",
					"/cnb/lifecycle/analyzer",
					"-layers=/layers",
					"-analyzed=/layers/analyzed.toml"}, tags...),
					"-previous-image=someimage/name@sha256:previous", "someimage/name"), analyzeContainer.Args)
			})

			it("configures restore step", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				restoreContainer := pod.Spec.InitContainers[3]
				assert.Equal(t, "restore", restoreContainer.Name)
				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, restoreContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})
				assert.Subset(t, restoreContainer.Env, []corev1.EnvVar{
					{
						Name:  "USERPROFILE",
						Value: "/builder/home",
					},
				})
				assert.Equal(t, []string{"/networkWait/network-wait-launcher"}, restoreContainer.Command)
				assert.Equal(t, []string{
					dnsProbeHost,
					"--",
					"/cnb/lifecycle/restorer",
					"-group=/layers/group.toml",
					"-layers=/layers",
					"-analyzed=/layers/analyzed.toml"},
					restoreContainer.Args)
			})

			it("configures restore step with older api", func() {
				pod, err := build.BuildPod(config, oldBuildContext)
				require.NoError(t, err)

				restoreContainer := pod.Spec.InitContainers[3]
				assert.Equal(t, "restore", restoreContainer.Name)
				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, restoreContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})
				assert.Subset(t, restoreContainer.Env, []corev1.EnvVar{
					{
						Name:  "USERPROFILE",
						Value: "/builder/home",
					},
				})
				assert.Equal(t, []string{"/networkWait/network-wait-launcher"}, restoreContainer.Command)
				assert.Equal(t, []string{
					dnsProbeHost,
					"--",
					"/cnb/lifecycle/restorer",
					"-group=/layers/group.toml",
					"-layers=/layers"},
					restoreContainer.Args)
			})

			it("configures build step", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				buildContainer := pod.Spec.InitContainers[4]
				assert.Equal(t, "build", buildContainer.Name)
				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, buildContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})
				assert.Equal(t, []string{"/networkWait/network-wait-launcher"}, buildContainer.Command)
				assert.Equal(t, []string{
					dnsProbeHost,
					"--",
					"/cnb/lifecycle/builder",
					"-layers=/layers",
					"-app=/workspace",
					"-group=/layers/group.toml",
					"-plan=/layers/plan.toml"},
					buildContainer.Args)
			})

			it("configures export step", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				exportContainer := pod.Spec.InitContainers[5]
				assert.Equal(t, "export", exportContainer.Name)
				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, exportContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})
				assert.Subset(t, exportContainer.Env, []corev1.EnvVar{
					{
						Name:  "USERPROFILE",
						Value: "/builder/home",
					},
				})
				assert.Equal(t, []string{"/networkWait/network-wait-launcher"}, exportContainer.Command)
				assert.Equal(t, []string{
					dnsProbeHost,
					"--",
					"/cnb/lifecycle/exporter",
					"-layers=/layers",
					"-app=/workspace",
					"-group=/layers/group.toml",
					"-analyzed=/layers/analyzed.toml",
					"-project-metadata=/layers/project-metadata.toml",
					"-report=/var/report/report.toml",
					"someimage/name", "someimage/name:tag2", "someimage/name:tag3"},
					exportContainer.Args)
			})
			it("configures the completion container for notary on windows", func() {
				build.Spec.Notary = &corev1alpha1.NotaryConfig{V1: &corev1alpha1.NotaryV1Config{
					URL: "some-notary-server",
				}}

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				completionContainer := pod.Spec.Containers[0]
				assert.Equal(t, "completion", completionContainer.Name)
				assert.Equal(t, config.CompletionWindowsImage, completionContainer.Image)

				assert.Equal(t, []string{
					dnsProbeHost,
					"--",
					"/cnb/process/web",
					"-notary-v1-url=some-notary-server",
					"-basic-git=git-secret-1=https://github.com",
					"-ssh-git=git-secret-2=https://bitbucket.com",
					"-basic-docker=docker-secret-1=acr.io",
					"-dockerconfig=docker-secret-2",
					"-dockercfg=docker-secret-3",
					"-cosign-annotations=buildTimestamp=19440606.133000",
					"-cosign-annotations=buildNumber=12",
				}, completionContainer.Args)

				assert.Equal(t, "/networkWait/network-wait-launcher", completionContainer.Command[0])
				assert.Subset(t, completionContainer.Env, []corev1.EnvVar{
					{
						Name:  "USERPROFILE",
						Value: "/builder/home",
					},
				})
				assert.Subset(t, pod.Spec.Volumes, []corev1.Volume{
					{
						Name: "network-wait-launcher-dir",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				})
				assert.Subset(t, completionContainer.VolumeMounts, []corev1.VolumeMount{
					{
						Name:      "network-wait-launcher-dir",
						MountPath: "/networkWait",
					},
				})
			})

			it("configures the completion container on windows", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				completionContainer := pod.Spec.Containers[0]
				assert.Equal(t, config.CompletionWindowsImage, completionContainer.Image)

				assert.Equal(t, []string{
					dnsProbeHost,
					"--",
					"/cnb/process/web",
					"-basic-git=git-secret-1=https://github.com",
					"-ssh-git=git-secret-2=https://bitbucket.com",
					"-basic-docker=docker-secret-1=acr.io",
					"-dockerconfig=docker-secret-2",
					"-dockercfg=docker-secret-3",
					"-cosign-annotations=buildTimestamp=19440606.133000",
					"-cosign-annotations=buildNumber=12",
				}, completionContainer.Args)

				assert.Subset(t, completionContainer.Env, []corev1.EnvVar{
					{
						Name:  "USERPROFILE",
						Value: "/builder/home",
					},
				})
			})

			it("does not use volume cache on windows", func() {
				buildContext.BuildPodBuilderConfig.OS = "linux"
				build.Spec.Cache.Volume.ClaimName = "non-empty"

				podWithCache, _ := build.BuildPod(config, buildContext)

				buildContext.BuildPodBuilderConfig.OS = "windows"
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Len(t, pod.Spec.Volumes, len(podWithCache.Spec.Volumes)-1)
			})

			it("does use registry cache on windows", func() {
				buildContext.BuildPodBuilderConfig.OS = "windows"
				build.Spec.Cache = &buildapi.BuildCacheConfig{
					Registry: &buildapi.RegistryCache{Tag: "some-tag"},
				}
				build.Spec.LastBuild.Cache = buildapi.BuildCache{Image: "last-cache-image"}

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Equal(t, "-cache-image=last-cache-image", pod.Spec.InitContainers[3].Args[5])
				assert.Equal(t, "-cache-image=some-tag", pod.Spec.InitContainers[5].Args[8])
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
		if volume.Name == fmt.Sprintf(buildapi.SecretTemplateName, secretName) {
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
