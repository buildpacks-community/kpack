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

		var (
			fakeBuilderImage *registryfakes.FakeRemoteImage
			generator        *buildpod.Generator
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

		fakeRemoteImageFactory := registryfakes.NewFakeImageFactory()

		build := &v1alpha1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "simple-build",
				Namespace: namespace,
			},
			Spec: v1alpha1.BuildSpec{
				Tags: []string{
					"gcr.io/builder",
					"additional/names",
				},
				Builder: v1alpha1.BuildBuilderSpec{
					Image: "some/builder",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "secrets"},
					},
				},
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

		it.Before(func() {
			fakeBuilderImage = registryfakes.NewFakeRemoteImage("some/builder", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
			require.NoError(t, fakeBuilderImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
			require.NoError(t, fakeBuilderImage.SetLabel(cnb.BuilderMetadataLabel, `{ "stack": { "runImage": { "image": "some-registry.io/run-image"} } }`))
			require.NoError(t, fakeBuilderImage.SetEnv("CNB_USER_ID", "1234"))
			require.NoError(t, fakeBuilderImage.SetEnv("CNB_GROUP_ID", "5678"))
			require.NoError(t, fakeRemoteImageFactory.AddImage(fakeBuilderImage, registry.SecretRef{
				Namespace:        namespace,
				ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
			}))

			fakeRunImage := registryfakes.NewFakeRemoteImage("some-registry.io/run-image", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
			require.NoError(t, fakeRunImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
			require.NoError(t, fakeRemoteImageFactory.AddImage(fakeRunImage, registry.SecretRef{
				Namespace:        namespace,
				ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
			}))

			buildPodConfig := v1alpha1.BuildPodImages{}

			generator = &buildpod.Generator{
				BuildPodConfig:     buildPodConfig,
				K8sClient:          fakeK8sClient,
				RemoteImageFactory: fakeRemoteImageFactory,
			}
		})

		it("returns pod config with secrets on build's service account", func() {
			pod, err := generator.Generate(build)
			require.NoError(t, err)

			expectedPod, err := build.BuildPod(v1alpha1.BuildPodImages{}, []corev1.Secret{
				*gitSecret,
				*dockerSecret,
			}, v1alpha1.BuildPodBuilderConfig{
				BuilderSpec: v1alpha1.BuildBuilderSpec{
					Image:            "some/builder",
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: "secrets"}},
				},
				StackID:  "some.stack.id",
				RunImage: "some-registry.io/run-image",
				Uid:      1234,
				Gid:      5678,
			})
			require.NoError(t, err)
			require.Equal(t, expectedPod, pod)
		})

		when("the build has mirrors from the builder metadata", func() {
			it.Before(func() {
				require.NoError(t, fakeBuilderImage.SetLabel(cnb.BuilderMetadataLabel, `{ "stack": { "runImage": { "image": "some-registry.io/run-image", "mirrors": ["wrong-repo.com/other-run", "some-repo.com/run-mirror"]} } }`))
			})

			it("selects the correct run image from the list of mirrors", func() {
				fakeRunImage := registryfakes.NewFakeRemoteImage("some-repo.com/run-mirror", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
				require.NoError(t, fakeRunImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
				require.NoError(t, fakeRemoteImageFactory.AddImage(fakeRunImage, registry.SecretRef{
					Namespace:        namespace,
					ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
				}))

				build.Spec.Tags[0] = "some-repo.com/optimize-image-stuff"

				pod, err := generator.Generate(build)
				require.NoError(t, err)

				expectedPod, err := build.BuildPod(v1alpha1.BuildPodImages{}, []corev1.Secret{
					*gitSecret,
					*dockerSecret,
				}, v1alpha1.BuildPodBuilderConfig{
					BuilderSpec: v1alpha1.BuildBuilderSpec{
						Image:            "some/builder",
						ImagePullSecrets: []corev1.LocalObjectReference{{Name: "secrets"}},
					},
					StackID:  "some.stack.id",
					RunImage: "some-repo.com/run-mirror",
					Uid:      1234,
					Gid:      5678,
				})
				require.NoError(t, err)
				require.Equal(t, expectedPod, pod)
			})

			when("and the build has mirrors from the build spec", func() {
				build.Spec.Builder.RunImage = &v1alpha1.RunImage{
					Image: "not-optimal-repo.io/run",
					Mirrors: []v1alpha1.Mirror{
						{Image: "wrong-repo.com/other-run"},
						{Image: "incorrect-repo.com/different-run"},
					},
				}

				it("can still match metadata mirrors if they are the best fit", func() {
					fakeRunImage := registryfakes.NewFakeRemoteImage("some-repo.com/run-mirror", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
					require.NoError(t, fakeRunImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
					require.NoError(t, fakeRemoteImageFactory.AddImage(fakeRunImage, registry.SecretRef{
						Namespace:        namespace,
						ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
					}))

					build.Spec.Tags[0] = "some-repo.com/optimize-image-stuff"

					pod, err := generator.Generate(build)
					require.NoError(t, err)

					expectedPod, err := build.BuildPod(v1alpha1.BuildPodImages{}, []corev1.Secret{
						*gitSecret,
						*dockerSecret,
					}, v1alpha1.BuildPodBuilderConfig{
						BuilderSpec: v1alpha1.BuildBuilderSpec{
							Image:            "some/builder",
							ImagePullSecrets: []corev1.LocalObjectReference{{Name: "secrets"}},
						},
						StackID:  "some.stack.id",
						RunImage: "some-repo.com/run-mirror",
						Uid:      1234,
						Gid:      5678,
					})
					require.NoError(t, err)
					require.Equal(t, expectedPod, pod)
				})
			})
		})

		when("the build has mirrors from the build spec", func() {
			it("selects the correct run image from the list of mirrors", func() {
				build.Spec.Builder.RunImage = &v1alpha1.RunImage{
					Image: "not-optimal-repo.io/run",
					Mirrors: []v1alpha1.Mirror{
						{Image: "wrong-repo.com/other-run"},
						{Image: "incorrect-repo.com/different-run"},
						{Image: "some-repo.com/run-mirror"},
					},
				}

				fakeRunImage := registryfakes.NewFakeRemoteImage("some-repo.com/run-mirror", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
				require.NoError(t, fakeRunImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
				require.NoError(t, fakeRemoteImageFactory.AddImage(fakeRunImage, registry.SecretRef{
					Namespace:        namespace,
					ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
				}))

				build.Spec.Tags[0] = "some-repo.com/optimize-image-stuff"

				pod, err := generator.Generate(build)
				require.NoError(t, err)

				expectedPod, err := build.BuildPod(v1alpha1.BuildPodImages{}, []corev1.Secret{
					*gitSecret,
					*dockerSecret,
				}, v1alpha1.BuildPodBuilderConfig{
					BuilderSpec: v1alpha1.BuildBuilderSpec{
						Image:            "some/builder",
						ImagePullSecrets: []corev1.LocalObjectReference{{Name: "secrets"}},
					},
					StackID:  "some.stack.id",
					RunImage: "some-repo.com/run-mirror",
					Uid:      1234,
					Gid:      5678,
				})
				require.NoError(t, err)
				require.Equal(t, expectedPod, pod)
			})

			it("selects the run image when none of the mirrors match", func() {
				build.Spec.Builder.RunImage = &v1alpha1.RunImage{
					Image: "some-repo.com/run@sha256:3c09636ec258accd512f929afb25e464bee6720e3ec1b761ba2b372edf6f47e1",
					Mirrors: []v1alpha1.Mirror{
						{Image: "wrong-repo.com/other-run"},
						{Image: "incorrect-repo.com/different-run"},
						{Image: "not-optimal-repo.io/run"},
					},
				}

				fakeRunImage := registryfakes.NewFakeRemoteImage("some-repo.com/run-mirror", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
				require.NoError(t, fakeRunImage.SetLabel(metadata.StackMetadataLabel, "some.stack.id"))
				require.NoError(t, fakeRemoteImageFactory.AddImage(fakeRunImage, registry.SecretRef{
					Namespace:        namespace,
					ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
				}))

				build.Spec.Tags[0] = "some-repo.com/optimize-image-stuff"

				pod, err := generator.Generate(build)
				require.NoError(t, err)

				expectedPod, err := build.BuildPod(v1alpha1.BuildPodImages{}, []corev1.Secret{
					*gitSecret,
					*dockerSecret,
				}, v1alpha1.BuildPodBuilderConfig{
					BuilderSpec: v1alpha1.BuildBuilderSpec{
						Image:            "some/builder",
						ImagePullSecrets: []corev1.LocalObjectReference{{Name: "secrets"}},
					},
					StackID:  "some.stack.id",
					RunImage: "some-repo.com/run",
					Uid:      1234,
					Gid:      5678,
				})
				require.NoError(t, err)
				require.Equal(t, expectedPod, pod)
			})
		})
	})
}
