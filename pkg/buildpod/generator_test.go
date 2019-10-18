package buildpod_test

import (
	"testing"

	"github.com/buildpack/lifecycle/metadata"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestGenerator(t *testing.T) {
	spec.Run(t, "Generator", testGenerator)
}

func testGenerator(t *testing.T, when spec.G, it spec.S) {
	when("Generate", func() {
		const (
			serviceAccountName = "serviceAccountName"
			namespace          = "some-namespace"
		)

		gitSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-secret-1",
				Namespace: namespace,
				Annotations: map[string]string{
					v1alpha1.GITSecretAnnotationPrefix: "https://github.com",
				},
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		}

		dockerSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-secret-1",
				Namespace: namespace,
				Annotations: map[string]string{
					v1alpha1.DOCKERSecretAnnotationPrefix: "https://gcr.io",
				},
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		}

		ignoredSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ignored-secret",
				Namespace: namespace,
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		}

		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      serviceAccountName,
			},
			Secrets: []corev1.ObjectReference{
				{
					Kind: "secret",
					Name: "git-secret-1",
				},
				{
					Kind: "secret",
					Name: "docker-secret-1",
				},
			},
		}
		fakeK8sClient := fake.NewSimpleClientset(serviceAccount, dockerSecret, gitSecret, ignoredSecret)

		builder := &v1alpha1.Builder{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.BuilderKind,
				APIVersion: "v1alpha1",
			},
		}
		clusterBuilder := &v1alpha1.ClusterBuilder{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.ClusterBuilderKind,
				APIVersion: "v1alpha1",
			},
		}

		it("returns pod config with secrets on build's service account", func() {
			fakeRemoteImageFactory := &registryfakes.FakeRemoteImageFactory{}
			fakeImage := registryfakes.NewFakeRemoteImage("some/builder", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
			require.NoError(t, fakeImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
			require.NoError(t, fakeImage.SetLabel(cnb.BuilderMetadataLabel, `{ "stack": { "runImage": { "image": "some-registry.io/run-image"} } }`))
			require.NoError(t, fakeImage.SetEnv("CNB_USER_ID", "1234"))
			require.NoError(t, fakeImage.SetEnv("CNB_GROUP_ID", "5678"))

			fakeRemoteImageFactory.NewRemoteReturns(fakeImage, nil)

			buildPodConfig := v1alpha1.BuildPodImages{}
			generator := &buildpod.Generator{
				BuildPodConfig:     buildPodConfig,
				K8sClient:          fakeK8sClient,
				RemoteImageFactory: fakeRemoteImageFactory,
			}

			build := &v1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple-build",
					Namespace: namespace,
				},
				Spec: v1alpha1.BuildSpec{
					Tags: []string{
						"builderImage/name",
						"additional/names",
					},
					Builder:        builder.BuildBuilderSpec(),
					ServiceAccount: serviceAccountName,
					Source: v1alpha1.SourceConfig{
						Git: &v1alpha1.Git{
							URL:      "http://www.google.com",
							Revision: "master",
						},
					},
					CacheName: "some-cache-name",
					Env: []corev1.EnvVar{
						{
							Name:  "ENV",
							Value: "NAME",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("256M"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("128M"),
						},
					},
				},
			}
			pod, err := generator.Generate(build)
			require.NoError(t, err)

			expectedPod, err := build.BuildPod(buildPodConfig, []corev1.Secret{
				*gitSecret,
				*dockerSecret,
			}, v1alpha1.BuildPodBuilderConfig{
				BuilderSpec: builder.BuildBuilderSpec(),
				StackID:     "some.stack.id",
				RunImage:    "some-registry.io/run-image",
				Uid:         1234,
				Gid:         5678,
			})
			require.NoError(t, err)
			require.Equal(t, expectedPod, pod)

			require.Equal(t, 1, fakeRemoteImageFactory.NewRemoteCallCount())

			builderImage, secretRef := fakeRemoteImageFactory.NewRemoteArgsForCall(0)
			require.Equal(t, builder.BuildBuilderSpec().Image, builderImage)
			require.Equal(t, registry.SecretRef{
				Namespace:        namespace,
				ImagePullSecrets: builder.BuildBuilderSpec().ImagePullSecrets,
			}, secretRef)
		})

		it("returns pod config with secrets on build's service account for a cluster builder", func() {
			fakeRemoteImageFactory := &registryfakes.FakeRemoteImageFactory{}
			fakeImage := registryfakes.NewFakeRemoteImage("some/builder", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
			require.NoError(t, fakeImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
			require.NoError(t, fakeImage.SetLabel(cnb.BuilderMetadataLabel, `{ "stack": { "runImage": { "image": "some-registry.io/run-image"} } }`))
			require.NoError(t, fakeImage.SetEnv("CNB_USER_ID", "1234"))
			require.NoError(t, fakeImage.SetEnv("CNB_GROUP_ID", "5678"))

			fakeRemoteImageFactory.NewRemoteReturns(fakeImage, nil)

			buildPodConfig := v1alpha1.BuildPodImages{}
			generator := &buildpod.Generator{
				BuildPodConfig:     buildPodConfig,
				K8sClient:          fakeK8sClient,
				RemoteImageFactory: fakeRemoteImageFactory,
			}

			build := &v1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "simple-build",
					Namespace: namespace,
				},
				Spec: v1alpha1.BuildSpec{
					Tags: []string{
						"builderImage/name",
						"additional/names",
					},
					Builder:        clusterBuilder.BuildBuilderSpec(),
					ServiceAccount: serviceAccountName,
					Source: v1alpha1.SourceConfig{
						Git: &v1alpha1.Git{
							URL:      "http://www.google.com",
							Revision: "master",
						},
					},
					CacheName: "some-cache-name",
					Env: []corev1.EnvVar{
						{
							Name:  "ENV",
							Value: "NAME",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("2"),
							corev1.ResourceMemory: resource.MustParse("256M"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("128M"),
						},
					},
				},
			}
			pod, err := generator.Generate(build)
			require.NoError(t, err)

			expectedPod, err := build.BuildPod(buildPodConfig, []corev1.Secret{
				*gitSecret,
				*dockerSecret,
			}, v1alpha1.BuildPodBuilderConfig{
				BuilderSpec: clusterBuilder.BuildBuilderSpec(),
				StackID:     "some.stack.id",
				RunImage:    "some-registry.io/run-image",
				Uid:         1234,
				Gid:         5678,
			})
			require.NoError(t, err)
			require.Equal(t, expectedPod, pod)

			require.Equal(t, 1, fakeRemoteImageFactory.NewRemoteCallCount())

			clusterBuilderImage, secretRef := fakeRemoteImageFactory.NewRemoteArgsForCall(0)
			require.Equal(t, clusterBuilder.BuildBuilderSpec().Image, clusterBuilderImage)
			require.Equal(t, registry.SecretRef{}, secretRef)
		})
	})
}
