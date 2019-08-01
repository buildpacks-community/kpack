package buildpod_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/buildpod"
)

func TestGenerator(t *testing.T) {
	spec.Run(t, "Generator", testGenerator)
}

func testGenerator(t *testing.T, when spec.G, it spec.S) {
	when("Generate", func() {
		const serviceAccountName = "serviceAccountName"

		gitSecret := &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
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
		}

		dockerSecret := &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name: "docker-secret-1",
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
			ObjectMeta: v1.ObjectMeta{
				Name: "ignored-secret",
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		}

		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "namespace",
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

		it("returns pod config with secrets on build's service account", func() {

			buildPodConfig := v1alpha1.BuildPodConfig{
				GitInitImage:   "git/init:image",
				BuildInitImage: "build/init:image",
				CredsInitImage: "creds/init:image",
				NopImage:       "no/op:image",
			}
			generator := &buildpod.Generator{
				BuildPodConfig: buildPodConfig,
				K8sClient:      fakeK8sClient,
			}

			build := &v1alpha1.Build{
				ObjectMeta: v1.ObjectMeta{
					Name: "simple-build",
				},
				Spec: v1alpha1.BuildSpec{
					Tag:            "image/name",
					Builder:        "builder/name",
					ServiceAccount: serviceAccountName,
					Source: v1alpha1.Source{
						Git: v1alpha1.Git{
							URL:      "http://www.google.com",
							Revision: "master",
						},
					},
					CacheName: "some-cache-name",
					AdditionalImageNames: []string{
						"additional/names",
					},
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
			})
			require.NoError(t, err)
			require.Equal(t, expectedPod, pod)
		})
	})
}
