package buildpod_test

import (
	"testing"

	"github.com/buildpack/lifecycle/metadata"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/duckbuilder"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
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
			keychainFactory = &registryfakes.FakeKeychainFactory{}
			imageFetcher    = registryfakes.NewFakeClient()
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

		builder := &duckbuilder.DuckBuilder{
			TypeMeta: metav1.TypeMeta{
				Kind:       v1alpha1.BuilderKind,
				APIVersion: "v1alpha1",
			},
			Status: v1alpha1.BuilderStatus{
				LatestImage: "some/builde@sha256:1234",
			},
		}

		it("returns pod config with secrets on build's service account", func() {
			secretRef := registry.SecretRef{
				Namespace:        namespace,
				ImagePullSecrets: builder.Spec.ImagePullSecrets,
			}
			keychain := &registryfakes.FakeKeychain{}
			keychainFactory.AddKeychainForSecretRef(t, secretRef, keychain)

			image := randomImage(t)
			image, _ = imagehelpers.SetStringLabel(image, metadata.StackMetadataLabel, "some.stack.id")
			image, _ = imagehelpers.SetStringLabel(image, cnb.BuilderMetadataLabel, `{ "stack": { "runImage": { "image": "some-registry.io/run-image"} } }`)
			image, _ = imagehelpers.SetEnv(image, "CNB_USER_ID=1234")
			image, _ = imagehelpers.SetEnv(image, "CNB_GROUP_ID=5678")
			imageFetcher.AddImage("some/builde@sha256:1234", image, "some/builder@sha256:1234", keychain)

			buildPodConfig := v1alpha1.BuildPodImages{}
			generator := &buildpod.Generator{
				BuildPodConfig:  buildPodConfig,
				K8sClient:       fakeK8sClient,
				KeychainFactory: keychainFactory,
				ImageFetcher:    imageFetcher,
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
		})
	})
}

func randomImage(t *testing.T) ggcrv1.Image {
	image, err := random.Image(5, 10)
	require.NoError(t, err)
	return image
}
