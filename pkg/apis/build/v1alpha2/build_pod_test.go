package v1alpha2_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
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

	var (
		build *buildapi.Build
	)

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
				Name: "secret.with.dots",
			},
			Type: corev1.SecretTypeDockerConfigJson,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "blob-secret",
				Annotations: map[string]string{
					buildapi.BlobSecretAnnotationPrefix: "blobstore.com",
				},
			},
			Type: corev1.SecretTypeOpaque,
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
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cosign-secret.with.dots",
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
			PlatformAPIs: []string{"0.7", "0.8", "0.9"},
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

	it.Before(func() {
		build = &buildapi.Build{
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
						URL:                  "giturl.com/git.git",
						Revision:             "gitrev1234",
						InitializeSubmodules: true,
					},
				},
				Cache: &buildapi.BuildCacheConfig{
					Volume: &buildapi.BuildPersistentVolumeCache{
						ClaimName: "some-cache-name",
					},
				},
				RunImage: buildapi.BuildSpecImage{
					Image: "builderregistry.io/run",
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
					{Name: "keyC", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "my-secret",
							},
							Key: "keyC",
						},
					}},
				},
				Resources: resources,
				LastBuild: &buildapi.LastBuild{
					Image:   previousAppImage,
					StackId: "com.builder.stack.io",
				},
				Tolerations:  []corev1.Toleration{{Key: "some-key"}},
				NodeSelector: map[string]string{"foo": "bar"},
				Affinity:     &corev1.Affinity{},
				CreationTime: "now",
			},
		}
	})

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
					"some/annotation":         "to-pass-through",
					"sidecar.istio.io/inject": "false",
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
				PlatformAPIs: []string{"0.7", "0.8", "0.9"},
				OS:           "linux",
			}.Uid, *pod.Spec.SecurityContext.RunAsUser)
			assert.Equal(t, buildapi.BuildPodBuilderConfig{
				StackID:      "com.builder.stack.io",
				RunImage:     "builderregistry.io/run",
				Uid:          2000,
				Gid:          3000,
				PlatformAPIs: []string{"0.7", "0.8", "0.9"},
				OS:           "linux",
			}.Gid, *pod.Spec.SecurityContext.RunAsGroup)
			assert.Equal(t, buildapi.BuildPodBuilderConfig{
				StackID:      "com.builder.stack.io",
				RunImage:     "builderregistry.io/run",
				Uid:          2000,
				Gid:          3000,
				PlatformAPIs: []string{"0.7", "0.8", "0.9"},
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
					Name: "binding-database",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "database",
						},
					},
				},
				corev1.Volume{
					Name: "binding-apm",
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
						Name:      "binding-database",
						MountPath: "/platform/bindings/database",
						ReadOnly:  true,
					},
					corev1.VolumeMount{
						Name:      "binding-apm",
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
				"-dockerconfig=secret.with.dots",
				"-imagepull=image-pull-1",
				"-imagepull=image-pull-2",
				"-imagepull=builder-pull-secret",
			}, pod.Spec.InitContainers[0].Args)

			assert.Subset(t,
				pod.Spec.InitContainers[0].VolumeMounts,
				[]corev1.VolumeMount{
					{
						Name:      "secret-volume-0",
						MountPath: "/var/build-secrets/git-secret-1",
					},
					{
						Name:      "secret-volume-1",
						MountPath: "/var/build-secrets/git-secret-2",
					},
					{
						Name:      "secret-volume-2",
						MountPath: "/var/build-secrets/docker-secret-1",
					},
					{
						Name:      "secret-volume-4",
						MountPath: "/var/build-secrets/docker-secret-2",
					},
					{
						Name:      "secret-volume-5",
						MountPath: "/var/build-secrets/docker-secret-3",
					},
					{
						Name:      "secret-volume-6",
						MountPath: "/var/build-secrets/secret.with.dots",
					},
					{
						Name:      "pull-secret-volume-0",
						MountPath: "/var/build-secrets/image-pull-1",
					},
					{
						Name:      "pull-secret-volume-1",
						MountPath: "/var/build-secrets/image-pull-2",
					},
					{
						Name:      "pull-secret-volume-2",
						MountPath: "/var/build-secrets/builder-pull-secret",
					},
				},
			)

		})

		it("configures prepare with blob credentials when using secret", func() {
			build.Spec.Source = corev1alpha1.SourceConfig{
				Blob: &corev1alpha1.Blob{
					URL:  "blobstore.com/source",
					Auth: "secret",
				},
			}

			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
			assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)

			assert.Contains(t, pod.Spec.InitContainers[0].Args, "-blob=blob-secret=blobstore.com")
			assert.Contains(t, pod.Spec.InitContainers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      "secret-volume-7",
					MountPath: "/var/build-secrets/blob-secret",
				},
			)
			assert.Contains(t, pod.Spec.InitContainers[0].Env,
				corev1.EnvVar{
					Name:  "BLOB_AUTH",
					Value: "true",
				},
			)
		})

		it("configures prepare with blob credentials when using helper", func() {
			build.Spec.Source = corev1alpha1.SourceConfig{
				Blob: &corev1alpha1.Blob{
					URL:  "blobstore.com/source",
					Auth: "helper",
				},
			}

			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
			assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)

			assert.NotContains(t, pod.Spec.InitContainers[0].Args, "-blob=blob-secret=blobstore.com")
			assert.NotContains(t, pod.Spec.InitContainers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      "secret-volume-7",
					MountPath: "/var/build-secrets/blob-secret",
				},
			)

			assert.Contains(t, pod.Spec.InitContainers[0].Env,
				corev1.EnvVar{
					Name:  "BLOB_AUTH",
					Value: "true",
				},
			)
		})

		it("configures prepare with the build configuration", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[0].Name, "prepare")
			assert.Equal(t, pod.Spec.InitContainers[0].Image, config.BuildInitImage)
			assert.Contains(t, pod.Spec.InitContainers[0].Env,
				corev1.EnvVar{
					Name:  "PLATFORM_ENV_keyA",
					Value: "valueA",
				})
			assert.Contains(t, pod.Spec.InitContainers[0].Env,
				corev1.EnvVar{
					Name:  "PLATFORM_ENV_keyB",
					Value: "valueB",
				})
			assert.Contains(t, pod.Spec.InitContainers[0].Env,
				corev1.EnvVar{
					Name: "PLATFORM_ENV_keyC",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "my-secret",
							},
							Key: "keyC",
						},
					},
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
			assert.Contains(t, pod.Spec.InitContainers[0].Env,
				corev1.EnvVar{
					Name:  "GIT_INITIALIZE_SUBMODULES",
					Value: fmt.Sprintf("%v", build.Spec.Source.Git.InitializeSubmodules),
				})
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
			assert.Contains(t, pod.Spec.InitContainers[2].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
			assert.Equal(t, pod.Spec.InitContainers[2].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
				"binding-database",
				"binding-apm",
			}, names(pod.Spec.InitContainers[2].VolumeMounts))
		})

		it("configures detect step with cnb bindings", func() {
			buildContext.Bindings = v1alpha1ServiceBindings
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[2].Name, "detect")
			assert.Contains(t, pod.Spec.InitContainers[2].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
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
			assert.Contains(t, pod.Spec.InitContainers[1].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
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
				"-analyzed=/layers/analyzed.toml",
				"-run-image=builderregistry.io/run",
			},
				tags...),
				"-previous-image="+build.Spec.LastBuild.Image, build.Tag()), pod.Spec.InitContainers[1].Args)
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
				"-analyzed=/layers/analyzed.toml",
				"-run-image=builderregistry.io/run",
			},
				tags...),
				build.Tag()), pod.Spec.InitContainers[1].Args)
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
			assert.Contains(t, pod.Spec.InitContainers[3].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
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

		it("configures build step", func() {
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[4].Name, "build")
			assert.Contains(t, pod.Spec.InitContainers[4].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
			assert.Equal(t, pod.Spec.InitContainers[4].Image, builderImage)
			assert.Equal(t, []string{
				"layers-dir",
				"platform-dir",
				"workspace-dir",
				"binding-database",
				"binding-apm",
			}, names(pod.Spec.InitContainers[4].VolumeMounts))
		})

		it("configures the build step with v1alpha1bindings", func() {
			buildContext.Bindings = v1alpha1ServiceBindings
			pod, err := build.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Equal(t, pod.Spec.InitContainers[4].Name, "build")
			assert.Contains(t, pod.Spec.InitContainers[4].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
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
			assert.Contains(t, pod.Spec.InitContainers[5].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
			_, ok := fetchEnvVar(pod.Spec.InitContainers[5].Env, "SOURCE_DATE_EPOCH")
			assert.Equal(t, true, ok)
			assert.Contains(t, pod.Spec.InitContainers[5].Env, corev1.EnvVar{Name: "CNB_RUN_IMAGE", Value: "builderregistry.io/run"})
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
			assert.Contains(t, pod.Spec.InitContainers[5].Env, corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.9"})
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

		when("registry cache is requested", func() {
			var (
				podWithVolumeCache *corev1.Pod
			)

			it.Before(func() {
				podWithVolumeCache, _ = build.BuildPod(config, buildContext)
				build.Spec.Cache.Volume = nil
				build.Spec.Cache.Registry = &buildapi.RegistryCache{Tag: "test-cache-image"}
			})
			when("first build", func() {
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
				it("adds the cache tag to the completion container", func() {
					podWithImageCache, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					exportContainer := podWithImageCache.Spec.Containers[0]
					assert.Contains(t, exportContainer.Env, corev1.EnvVar{Name: "CACHE_TAG", Value: "test-cache-image"})
				})
			})

			when("second build", func() {
				it.Before(func() {
					build.Spec.LastBuild = &buildapi.LastBuild{
						Cache: buildapi.BuildCache{
							Image: "test-cache-image@sha",
						},
					}
				})

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
				it("adds the cache tag to the completion container", func() {
					podWithImageCache, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					exportContainer := podWithImageCache.Spec.Containers[0]
					assert.Contains(t, exportContainer.Env, corev1.EnvVar{Name: "CACHE_TAG", Value: "test-cache-image"})
				})
			})
		})

		when("ImageTag is empty", func() {
			it.Before(func() {
				build.Spec.Cache.Registry = &buildapi.RegistryCache{Tag: ""}
			})

			it("does not add the cache to analyze container", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				analyzeContainer := pod.Spec.InitContainers[2]
				assert.NotContains(t, analyzeContainer.Args, "-cache-image")
			})
			it("does not add the cache to restore container", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				restoreContainer := pod.Spec.InitContainers[3]
				assert.NotContains(t, restoreContainer.Args, "-cache-image")
			})
			it("does not add the cache to export container", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				exportContainer := pod.Spec.InitContainers[5]
				assert.NotContains(t, exportContainer.Args, "-cache-image")
			})
		})

		it("creates a pod without cache volume when cache is nil", func() {
			buildCopy := build.DeepCopy()
			podWithCache, _ := buildCopy.BuildPod(config, buildContext)
			buildCopy.Spec.Cache = nil
			pod, err := buildCopy.BuildPod(config, buildContext)
			require.NoError(t, err)

			assert.Len(t, pod.Spec.Volumes, len(podWithCache.Spec.Volumes)-1)
		})

		when("CacheName is empty", func() {
			var podWithCache *corev1.Pod
			it.Before(func() {
				podWithCache, _ = build.BuildPod(config, buildContext)
				build.Spec.Cache.Volume = &buildapi.BuildPersistentVolumeCache{ClaimName: ""}
			})

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

		when("no supported platform apis are available", func() {
			buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.2", "0.999"}

			it("returns an error", func() {
				_, err := build.BuildPod(config, buildContext)
				require.EqualError(t, err, "unsupported builder platform API versions: 0.2,0.999")
			})
		})

		when("creating a rebase pod", func() {
			it.Before(func() {
				build.Annotations[buildapi.BuildReasonAnnotation] = buildapi.BuildReasonStack
				build.Annotations[buildapi.BuildChangesAnnotation] = "some-stack-change"
			})

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
						"sidecar.istio.io/inject":       "false",
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
						Name: "secret-volume-2",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "docker-secret-1",
							},
						},
					},
					{
						Name: "secret-volume-4",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "docker-secret-2",
							},
						},
					},
					{
						Name: "secret-volume-5",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "docker-secret-3",
							},
						},
					},
					{
						Name: "secret-volume-6",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "secret.with.dots",
							},
						},
					},
					{
						Name: "pull-secret-volume-0",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "image-pull-1",
							},
						},
					},
					{
						Name: "pull-secret-volume-1",
						VolumeSource: corev1.VolumeSource{
							Secret: &corev1.SecretVolumeSource{
								SecretName: "image-pull-2",
							},
						},
					},
					{
						Name: "pull-secret-volume-2",
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
						Command:   []string{"/cnb/process/rebase"},
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
							"-dockerconfig=secret.with.dots",
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
								Name:      "secret-volume-2",
								MountPath: "/var/build-secrets/docker-secret-1",
							},
							{
								Name:      "secret-volume-4",
								MountPath: "/var/build-secrets/docker-secret-2",
							},
							{
								Name:      "secret-volume-5",
								MountPath: "/var/build-secrets/docker-secret-3",
							},
							{
								Name:      "secret-volume-6",
								MountPath: "/var/build-secrets/secret.with.dots",
							},
							{
								Name:      "pull-secret-volume-0",
								MountPath: "/var/build-secrets/image-pull-1",
							},
							{
								Name:      "pull-secret-volume-1",
								MountPath: "/var/build-secrets/image-pull-2",
							},
							{
								Name:      "pull-secret-volume-2",
								MountPath: "/var/build-secrets/builder-pull-secret",
							},
							{
								Name:      "report-dir",
								MountPath: "/var/report",
							},
						},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             boolPointer(true),
							AllowPrivilegeEscalation: boolPointer(false),
							Privileged:               boolPointer(false),
							SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					},
				}, pod.Spec.InitContainers)

				require.Equal(t, []corev1.Container{
					{
						Name:      "completion",
						Command:   []string{"/cnb/process/completion"},
						Image:     config.CompletionImage,
						Resources: build.Spec.Resources,
						Env: []corev1.EnvVar{
							{Name: "CACHE_TAG", Value: ""},
							{Name: "TERMINATION_MESSAGE_PATH", Value: "/tmp/termination-log"},
						},
						Args: []string{
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
							"-dockerconfig=secret.with.dots",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
						},
						ImagePullPolicy: corev1.PullIfNotPresent,
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "report-dir",
								MountPath: "/var/report",
							},
							{
								Name:      "notary-dir",
								MountPath: "/var/notary/v1",
								ReadOnly:  true,
							},
							{
								Name:      "secret-volume-2",
								MountPath: "/var/build-secrets/docker-secret-1",
							},
							{
								Name:      "secret-volume-4",
								MountPath: "/var/build-secrets/docker-secret-2",
							},
							{
								Name:      "secret-volume-5",
								MountPath: "/var/build-secrets/docker-secret-3",
							},
							{
								Name:      "secret-volume-6",
								MountPath: "/var/build-secrets/secret.with.dots",
							},
						},
						TerminationMessagePath:   "/tmp/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             boolPointer(true),
							AllowPrivilegeEscalation: boolPointer(false),
							Privileged:               boolPointer(false),
							SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					},
				}, pod.Spec.Containers)
			})

			when("cosign secrets are present on the build", func() {
				it("skips invalid secrets", func() {
					buildContext.Secrets = append(secrets, cosignInvalidSecrets...)
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					assert.NotNil(t, pod.Spec.Containers[0])
					assert.NotNil(t, pod.Spec.Containers[0].Command[0])
					assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

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
					}
					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      "secret-volume-9",
						MountPath: "/var/build-secrets/cosign/cosign-secret-1",
					})
					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      "secret-volume-10",
						MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-1",
					})
					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      "secret-volume-11",
						MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-2",
					})

					require.Equal(t,
						[]string{
							"-basic-docker=docker-secret-1=acr.io",
							"-dockerconfig=docker-secret-2",
							"-dockercfg=docker-secret-3",
							"-dockerconfig=secret.with.dots",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
							"-cosign-docker-media-types=cosign-secret.with.dots=1",
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
							"-dockerconfig=secret.with.dots",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-annotations=customAnnotationKey=customAnnotationValue",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
							"-cosign-docker-media-types=cosign-secret.with.dots=1",
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
								"-dockerconfig=secret.with.dots",
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

				it("errs if platformApi does not support report.toml", func() {
					buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

					_, err := build.BuildPod(config, buildContext)
					require.EqualError(t, err, "unsupported builder platform API versions: 0.3,0.2")
				})

				it("sets up the completion image to sign the image", func() {
					pod, err := build.BuildPod(config, buildContext)
					require.NoError(t, err)

					assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

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

					assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

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

					assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

					validSecrets := []string{
						"cosign-secret-1",
						"cosign-secret-no-password-1",
						"cosign-secret-no-password-2",
					}

					for _, secretName := range validSecrets {
						assertSecretPresent(t, pod, secretName)
					}
					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      "secret-volume-9",
						MountPath: "/var/build-secrets/cosign/cosign-secret-1",
					})
					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      "secret-volume-10",
						MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-1",
					})
					require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
						Name:      "secret-volume-11",
						MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-2",
					})
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
							"-dockerconfig=secret.with.dots",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
							"-cosign-docker-media-types=cosign-secret.with.dots=1",
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
							"-dockerconfig=secret.with.dots",
							"-cosign-annotations=buildTimestamp=19440606.133000",
							"-cosign-annotations=buildNumber=12",
							"-cosign-annotations=customAnnotationKey=customAnnotationValue",
							"-cosign-repositories=cosign-secret-1=testRepository.com/fake-project-1",
							"-cosign-docker-media-types=cosign-secret-1=1",
							"-cosign-repositories=cosign-secret-no-password-1=testRepository.com/fake-project-2",
							"-cosign-docker-media-types=cosign-secret-no-password-2=1",
							"-cosign-docker-media-types=cosign-secret.with.dots=1",
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
				assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

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
				}
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "secret-volume-9",
					MountPath: "/var/build-secrets/cosign/cosign-secret-1",
				})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "secret-volume-10",
					MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-1",
				})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "secret-volume-11",
					MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-2",
				})

				expectedArgs := []string{
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
				}
				for _, a := range expectedArgs {
					require.Contains(t, pod.Spec.Containers[0].Args, a)
				}
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
				expectedArgs := []string{
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
				}
				for _, a := range expectedArgs {
					require.Contains(t, pod.Spec.Containers[0].Args, a)
				}
			})
		})

		when("a notary config is present on the build", func() {
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

			it("errs if platformApi does not support report.toml", func() {
				buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.3", "0.2"}

				_, err := build.BuildPod(config, buildContext)
				require.EqualError(t, err, "unsupported builder platform API versions: 0.3,0.2")
			})

			it("sets up the completion image to sign the image", func() {
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

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

				assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

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

				assert.Equal(t, "/cnb/process/completion", pod.Spec.Containers[0].Command[0])

				validSecrets := []string{
					"cosign-secret-1",
					"cosign-secret-no-password-1",
					"cosign-secret-no-password-2",
				}

				for _, secretName := range validSecrets {
					assertSecretPresent(t, pod, secretName)
				}
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "secret-volume-9",
					MountPath: "/var/build-secrets/cosign/cosign-secret-1",
				})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "secret-volume-10",
					MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-1",
				})
				require.Contains(t, pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
					Name:      "secret-volume-11",
					MountPath: "/var/build-secrets/cosign/cosign-secret-no-password-2",
				})

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

				expectedArgs := []string{
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
				}
				for _, a := range expectedArgs {
					require.Contains(t, pod.Spec.Containers[0].Args, a)
				}
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

				expectedArgs := []string{
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
				}
				for _, a := range expectedArgs {
					require.Contains(t, pod.Spec.Containers[0].Args, a)
				}
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
					"-dockerconfig=secret.with.dots",
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
					"-analyzed=/layers/analyzed.toml", "-run-image=builderregistry.io/run"}, tags...),
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
					"/cnb/process/completion",
					"-notary-v1-url=some-notary-server",
					"-basic-git=git-secret-1=https://github.com",
					"-ssh-git=git-secret-2=https://bitbucket.com",
					"-basic-docker=docker-secret-1=acr.io",
					"-dockerconfig=docker-secret-2",
					"-dockercfg=docker-secret-3",
					"-dockerconfig=secret.with.dots",
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
					"/cnb/process/completion",
					"-basic-git=git-secret-1=https://github.com",
					"-ssh-git=git-secret-2=https://bitbucket.com",
					"-basic-docker=docker-secret-1=acr.io",
					"-dockerconfig=docker-secret-2",
					"-dockercfg=docker-secret-3",
					"-dockerconfig=secret.with.dots",
					"-cosign-annotations=buildTimestamp=19440606.133000",
					"-cosign-annotations=buildNumber=12",
				}, completionContainer.Args)

				assert.Equal(t, completionContainer.Env, []corev1.EnvVar{
					{Name: "USERPROFILE", Value: "/builder/home"},
					{Name: "CACHE_TAG", Value: ""},
					{Name: "TERMINATION_MESSAGE_PATH", Value: "/tmp/termination-log"},
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

		when("selecting platform api version", func() {
			it("chooses the configured maximum version when less than highest version from builder", func() {
				buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8"}

				platformApi, err := semver.NewVersion("0.7")
				require.NoError(t, err)
				buildContext.MaximumPlatformApiVersion = platformApi

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				expectedEnv := corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.7"}

				for _, container := range pod.Spec.InitContainers {
					if envVar, ok := fetchEnvVar(container.Env, expectedEnv.Name); ok {
						assert.Equal(t, expectedEnv, envVar)
					}
				}
			})
			it("chooses the highest version from builder when less than configured maximum version", func() {
				buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8"}

				platformApi, err := semver.NewVersion("1.0")
				require.NoError(t, err)
				buildContext.MaximumPlatformApiVersion = platformApi

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				expectedEnv := corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"}
				for _, container := range pod.Spec.InitContainers {
					if envVar, ok := fetchEnvVar(container.Env, expectedEnv.Name); ok {
						assert.Equal(t, expectedEnv, envVar)
					}
				}
			})
			it("chooses the highest version from builder when maximum version is not configured", func() {
				buildContext.BuildPodBuilderConfig.PlatformAPIs = []string{"0.2", "0.3", "0.4", "0.5", "0.6", "0.7", "0.8"}

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				expectedEnv := corev1.EnvVar{Name: "CNB_PLATFORM_API", Value: "0.8"}
				for _, container := range pod.Spec.InitContainers {
					if envVar, ok := fetchEnvVar(container.Env, expectedEnv.Name); ok {
						assert.Equal(t, expectedEnv, envVar)
					}
				}
			})
		})

		when("complying with the restricted pod security standard", func() {
			// enforces https://kubernetes.io/docs/concepts/security/pod-security-standards/#restricted
			var pod *corev1.Pod
			var rebasePod *corev1.Pod
			var err error

			it.Before(func() {
				pod, err = build.BuildPod(config, buildContext)
				require.NoError(t, err)

				build.Annotations[buildapi.BuildReasonAnnotation] = buildapi.BuildReasonStack
				build.Annotations[buildapi.BuildChangesAnnotation] = "some-stack-change"
				rebasePod, err = build.BuildPod(config, buildContext)
				require.NoError(t, err)
			})

			it("has non nil security contexts", func() {
				require.NotNil(t, pod.Spec.SecurityContext)
				require.NotNil(t, rebasePod.Spec.SecurityContext)
				validateNonNilSecurityContexts := func(containers []corev1.Container) {
					for _, container := range containers {
						require.NotNil(t, container.SecurityContext, container.Name)
					}
				}

				validateNonNilSecurityContexts(pod.Spec.InitContainers)
				validateNonNilSecurityContexts(pod.Spec.Containers)
				validateNonNilSecurityContexts(rebasePod.Spec.InitContainers)
				validateNonNilSecurityContexts(rebasePod.Spec.Containers)
			})

			it("disallows sharing of host namespaces", func() {
				validateHostNetwork := func(pod *corev1.Pod) {
					assert.False(t, pod.Spec.HostNetwork)
					assert.False(t, pod.Spec.HostPID)
					assert.False(t, pod.Spec.HostIPC)
				}

				validateHostNetwork(pod)
				validateHostNetwork(rebasePod)
			})
			it("disallows host ports", func() {
				validateNoPrivilegeEscalation := func(containers []corev1.Container) {
					for _, container := range containers {
						for _, port := range container.Ports {
							assert.Nil(t, port.HostPort)
						}
					}
				}

				validateNoPrivilegeEscalation(pod.Spec.InitContainers)
				validateNoPrivilegeEscalation(pod.Spec.Containers)
				validateNoPrivilegeEscalation(rebasePod.Spec.InitContainers)
				validateNoPrivilegeEscalation(rebasePod.Spec.Containers)
			})
			it("only uses allowed app armor values", func() {
				validateAppArmor := func(pod *corev1.Pod) {
					for key, value := range pod.Annotations {
						if strings.HasPrefix(key, corev1.AppArmorBetaContainerAnnotationKeyPrefix) {
							assert.Equal(t, corev1.AppArmorBetaProfileRuntimeDefault, value)
						}
					}
				}

				validateAppArmor(pod)
				validateAppArmor(rebasePod)
			})
			it("does not use se linux options", func() {

				validateContainerSELinux := func(containers []corev1.Container) {
					for _, container := range containers {
						assert.Nil(t, container.SecurityContext.SELinuxOptions)
					}
				}

				assert.Nil(t, pod.Spec.SecurityContext.SELinuxOptions)
				assert.Nil(t, rebasePod.Spec.SecurityContext.SELinuxOptions)
				validateContainerSELinux(pod.Spec.InitContainers)
				validateContainerSELinux(pod.Spec.Containers)
				validateContainerSELinux(rebasePod.Spec.InitContainers)
				validateContainerSELinux(rebasePod.Spec.Containers)
			})
			it("uses allowed volume types", func() {
				validateCorrectVolumeTypes := func(volumes []corev1.Volume) {
					for _, volume := range volumes {
						if volume.ConfigMap == nil &&
							volume.CSI == nil &&
							volume.DownwardAPI == nil &&
							volume.EmptyDir == nil &&
							volume.Ephemeral == nil &&
							volume.PersistentVolumeClaim == nil &&
							volume.Projected == nil &&
							volume.Secret == nil {
							assert.Fail(t, "invalid volume spec ")
						}
						assert.Nil(t, volume.HostPath)
					}
				}

				validateCorrectVolumeTypes(pod.Spec.Volumes)
				validateCorrectVolumeTypes(rebasePod.Spec.Volumes)

			})
			it("uses allowed proc mounts", func() {
				validateProcMount := func(containers []corev1.Container) {
					for _, container := range containers {
						if procMount := container.SecurityContext.ProcMount != nil; procMount {
							assert.Equal(t, corev1.DefaultProcMount, procMount)
						}
					}
				}

				validateProcMount(pod.Spec.InitContainers)
				validateProcMount(pod.Spec.Containers)
				validateProcMount(rebasePod.Spec.InitContainers)
				validateProcMount(rebasePod.Spec.Containers)
			})
			it("does not use sysctl", func() {
				assert.Empty(t, pod.Spec.SecurityContext.Sysctls)
				assert.Empty(t, rebasePod.Spec.SecurityContext.Sysctls)
			})
			it("does not allow privilege escalation or privileged containers", func() {
				validateNoPrivilegeEscalation := func(containers []corev1.Container) {
					for _, container := range containers {
						require.NotNil(t, container.SecurityContext.AllowPrivilegeEscalation, container.Name)
						assert.False(t, *container.SecurityContext.AllowPrivilegeEscalation)
						require.NotNil(t, container.SecurityContext.Privileged, container.Name)
						assert.False(t, *container.SecurityContext.Privileged)
					}
				}

				validateNoPrivilegeEscalation(pod.Spec.InitContainers)
				validateNoPrivilegeEscalation(pod.Spec.Containers)
				validateNoPrivilegeEscalation(rebasePod.Spec.InitContainers)
				validateNoPrivilegeEscalation(rebasePod.Spec.Containers)
			})
			it("runs as non root", func() {
				assert.True(t, *pod.Spec.SecurityContext.RunAsNonRoot)
				if pod.Spec.SecurityContext.RunAsUser != nil {
					assert.NotEqual(t, 0, *pod.Spec.SecurityContext.RunAsUser)
				}
				assert.True(t, *rebasePod.Spec.SecurityContext.RunAsNonRoot)
				if rebasePod.Spec.SecurityContext.RunAsUser != nil {
					assert.NotEqual(t, 0, *rebasePod.Spec.SecurityContext.RunAsUser)
				}

				validateNonRoot := func(containers []corev1.Container) {
					for _, container := range containers {
						require.NotNil(t, container.SecurityContext.RunAsNonRoot, container.Name)
						assert.True(t, *container.SecurityContext.RunAsNonRoot)
						if container.SecurityContext.RunAsUser != nil {
							assert.NotEqual(t, 0, *container.SecurityContext.RunAsUser)
						}
					}
				}

				validateNonRoot(pod.Spec.InitContainers)
				validateNonRoot(pod.Spec.Containers)
				validateNonRoot(rebasePod.Spec.InitContainers)
				validateNonRoot(rebasePod.Spec.Containers)
			})
			it("sets runtime/default seccomp profile", func() {
				expectedSeccomp := corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}

				assert.NotNil(t, pod.Spec.SecurityContext.SeccompProfile)
				assert.Equal(t, expectedSeccomp, *pod.Spec.SecurityContext.SeccompProfile)
				assert.NotNil(t, rebasePod.Spec.SecurityContext.SeccompProfile)
				assert.Equal(t, expectedSeccomp, *rebasePod.Spec.SecurityContext.SeccompProfile)

				validateSeccomp := func(containers []corev1.Container) {
					for _, container := range containers {
						require.NotNil(t, container.SecurityContext.SeccompProfile, container.Name)
						assert.Equal(t, expectedSeccomp, *container.SecurityContext.SeccompProfile)
					}
				}

				validateSeccomp(pod.Spec.InitContainers)
				validateSeccomp(pod.Spec.Containers)
				validateSeccomp(rebasePod.Spec.InitContainers)
				validateSeccomp(rebasePod.Spec.Containers)
			})
			it("drops all capabilities", func() {
				validateCapabilityDrop := func(containers []corev1.Container) {
					for _, container := range containers {
						require.NotNil(t, container.SecurityContext.SeccompProfile, container.Name)
						assert.Contains(t, container.SecurityContext.Capabilities.Drop, corev1.Capability("ALL"))
						assert.Empty(t, container.SecurityContext.Capabilities.Add)
					}
				}

				validateCapabilityDrop(pod.Spec.InitContainers)
				validateCapabilityDrop(pod.Spec.Containers)
				validateCapabilityDrop(rebasePod.Spec.InitContainers)
				validateCapabilityDrop(rebasePod.Spec.Containers)
			})
		})

		when("running builds in standard containers", func() {
			it("injects a pre start init container", func() {
				buildContext.InjectedSidecarSupport = true
				config.BuildWaiterImage = "some-image"

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				require.Len(t, pod.Spec.InitContainers, 1)
				preStartContainer := pod.Spec.InitContainers[0]
				assert.Equal(t, "pre-start", preStartContainer.Name)
				assert.Equal(t, "some-image", preStartContainer.Image)
				assert.Contains(t, preStartContainer.Args, "-mode=copy")
			})
			it("sets up build steps to run buildWaiter", func() {
				initContainerBuildPod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)
				containers := map[string]corev1.Container{}
				for _, container := range initContainerBuildPod.Spec.InitContainers {
					containers[container.Name] = container
				}
				for _, container := range initContainerBuildPod.Spec.Containers {
					containers[container.Name] = container
				}

				buildContext.InjectedSidecarSupport = true
				config.BuildWaiterImage = "some-image"
				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				require.Len(t, pod.Spec.Containers, 7)
				for i, container := range pod.Spec.Containers {
					assert.Equal(t, []string{"/buildWait/build-waiter"}, container.Command)
					assert.Equal(t, "-mode=wait", container.Args[0])
					assert.Equal(t, fmt.Sprintf("-done-file=/buildWait/%s", container.Name), container.Args[1])
					assert.Equal(t, "-error-file=/buildWait/error", container.Args[2])

					originalArgs := append(containers[container.Name].Command, containers[container.Name].Args...)
					assert.Equal(t, fmt.Sprintf("-execute=%s", strings.Join(originalArgs, " ")), container.Args[3])

					if i == 0 {
						assert.Equal(t, "-wait-file=/downward/sidecars-ready", container.Args[4])
					} else {
						assert.Equal(t, fmt.Sprintf("-wait-file=/buildWait/%s", pod.Spec.Containers[i-1].Name), container.Args[4])
					}

					assert.Contains(t, container.VolumeMounts, corev1.VolumeMount{Name: "build-wait-dir", MountPath: "/buildWait", ReadOnly: false})
				}
			})
			it("adds build-waiter volume to pod", func() {
				buildContext.InjectedSidecarSupport = true
				config.BuildWaiterImage = "some-image"

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Contains(t, pod.Spec.Volumes, corev1.Volume{Name: "build-wait-dir", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
			})
			it("adds downward api volume to pod", func() {
				buildContext.InjectedSidecarSupport = true

				pod, err := build.BuildPod(config, buildContext)
				require.NoError(t, err)

				assert.Contains(t, pod.Spec.Volumes,
					corev1.Volume{
						Name: "downward-api-dir",
						VolumeSource: corev1.VolumeSource{
							DownwardAPI: &corev1.DownwardAPIVolumeSource{
								Items: []corev1.DownwardAPIVolumeFile{
									{
										Path:     "sidecars-ready",
										FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.annotations['build.kpack.io/ready']"},
									},
								},
							},
						},
					},
				)
			})
		})
	})
}

func assertSecretPresent(t *testing.T, pod *corev1.Pod, secretName string) {
	t.Helper()
	assert.True(t, isSecretPresent(pod, secretName), fmt.Sprintf("secret '%s' not present in volumes '%v'", secretName, pod.Spec.Volumes))
}

func assertSecretNotPresent(t *testing.T, pod *corev1.Pod, secretName string) {
	t.Helper()
	assert.False(t, isSecretPresent(pod, secretName), fmt.Sprintf("secret '%s' present in volumes '%v' but should not be", secretName, pod.Spec.Volumes))
}

func isSecretPresent(pod *corev1.Pod, secretName string) bool {
	found := false
	for _, volume := range pod.Spec.Volumes {
		if reflect.DeepEqual(corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: secretName}}, volume.VolumeSource) {
			found = true
		}
	}
	return found
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

func fetchEnvVar(envVars []corev1.EnvVar, name string) (corev1.EnvVar, bool) {
	for _, envVar := range envVars {
		if envVar.Name == name {
			return envVar, true
		}
	}

	return corev1.EnvVar{}, false
}

func boolPointer(b bool) *bool {
	return &b
}
