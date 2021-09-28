package v1alpha2

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestBuildConversion(t *testing.T) {
	spec.Run(t, "testBuildConversion", testBuildConversion)
}

func testBuildConversion(t *testing.T, when spec.G, it spec.S) {
	when("converting to and from v1alpha1", func() {
		v1alpha2Build := &Build{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-build",
			},
			Spec: BuildSpec{
				Tags: []string{"tag"},
				Builder: corev1alpha1.BuildBuilderSpec{
					Image: "my-builder=image",
					ImagePullSecrets: []corev1.LocalObjectReference{{
						Name: "secret",
					}},
				},
				ServiceAccountName: "default",
				Cache: &BuildCacheConfig{
					Volume: &BuildPersistentVolumeCache{
						ClaimName: "some-claim-name",
					},
				},
				Env: []corev1.EnvVar{{
					Name:  "some-var",
					Value: "some-val",
				}},
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						"some-name": resource.MustParse("5M"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						"some-name": resource.MustParse("5M"),
					},
				},
				LastBuild: &LastBuild{
					Image:   "last-image",
					StackId: "my-stack",
				},
				Notary: &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "notary.com",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "notary",
						},
					},
				},
			},
			Status: BuildStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 0,
					Conditions:         nil,
				},
				BuildMetadata: corev1alpha1.BuildpackMetadataList{},
				Stack: corev1alpha1.BuildStack{
					RunImage: "some-run",
					ID:       "some-id",
				},
				LatestImage:    "some-latest",
				PodName:        "some-pod",
				StepStates:     []corev1.ContainerState{},
				StepsCompleted: []string{},
			},
		}
		v1alpha1Build := &v1alpha1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-build",
			},
			Spec: v1alpha1.BuildSpec{
				Tags: []string{"tag"},
				Builder: corev1alpha1.BuildBuilderSpec{
					Image: "my-builder=image",
					ImagePullSecrets: []corev1.LocalObjectReference{{
						Name: "secret",
					}},
				},
				ServiceAccount: "default",
				CacheName:      "some-claim-name",
				Bindings:       nil,
				Env: []corev1.EnvVar{{
					Name:  "some-var",
					Value: "some-val",
				}},
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						"some-name": resource.MustParse("5M"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						"some-name": resource.MustParse("5M"),
					},
				},
				LastBuild: &v1alpha1.LastBuild{
					Image:   "last-image",
					StackId: "my-stack",
				},
				Notary: &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "notary.com",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "notary",
						},
					},
				},
			},
			Status: v1alpha1.BuildStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 0,
					Conditions:         nil,
				},
				BuildMetadata: corev1alpha1.BuildpackMetadataList{},
				Stack: corev1alpha1.BuildStack{
					RunImage: "some-run",
					ID:       "some-id",
				},
				LatestImage:    "some-latest",
				PodName:        "some-pod",
				StepStates:     []corev1.ContainerState{},
				StepsCompleted: []string{},
			},
		}

		it("converts with git source", func() {
			v1alpha2Build.Spec.Source = corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "my-repo.com",
					Revision: "main",
				},
				SubPath: "my-sub-path",
			}
			v1alpha1Build.Spec.Source = corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "my-repo.com",
					Revision: "main",
				},
				SubPath: "my-sub-path",
			}

			testV1alpha1Build := &v1alpha1.Build{}
			err := v1alpha2Build.ConvertTo(context.TODO(), testV1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, testV1alpha1Build, v1alpha1Build)

			testV1alpha2Build := &Build{}
			err = testV1alpha2Build.ConvertFrom(context.TODO(), v1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Build, v1alpha2Build)
		})

		it("converts with registry source", func() {
			v1alpha2Build.Spec.Source = corev1alpha1.SourceConfig{
				Registry: &corev1alpha1.Registry{
					Image: "my-registry.com/image",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "my-image-secret",
						},
					},
				},
				SubPath: "my-sub-path",
			}
			v1alpha1Build.Spec.Source = corev1alpha1.SourceConfig{
				Registry: &corev1alpha1.Registry{
					Image: "my-registry.com/image",
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "my-image-secret",
						},
					},
				},
				SubPath: "my-sub-path",
			}

			testV1alpha1Build := &v1alpha1.Build{}
			err := v1alpha2Build.ConvertTo(context.TODO(), testV1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, testV1alpha1Build, v1alpha1Build)

			testV1alpha2Build := &Build{}
			err = testV1alpha2Build.ConvertFrom(context.TODO(), v1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Build, v1alpha2Build)
		})

		it("converts with blob source", func() {
			v1alpha2Build.Spec.Source = corev1alpha1.SourceConfig{
				Blob: &corev1alpha1.Blob{
					URL: "my-blob.com",
				},
				SubPath: "my-sub-path",
			}
			v1alpha1Build.Spec.Source = corev1alpha1.SourceConfig{
				Blob: &corev1alpha1.Blob{
					URL: "my-blob.com",
				},
				SubPath: "my-sub-path",
			}

			testV1alpha1Build := &v1alpha1.Build{}
			err := v1alpha2Build.ConvertTo(context.TODO(), testV1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, testV1alpha1Build, v1alpha1Build)

			testV1alpha2Build := &Build{}
			err = testV1alpha2Build.ConvertFrom(context.TODO(), v1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Build, v1alpha2Build)
		})

		it("handles nil values", func() {
			v1alpha2Build.Spec.LastBuild = nil
			v1alpha2Build.Spec.Notary = nil
			v1alpha2Build.Spec.Cache = nil
			testV1alpha1Build := &v1alpha1.Build{}
			err := v1alpha2Build.ConvertTo(context.TODO(), testV1alpha1Build)
			require.NoError(t, err)

			v1alpha1Build.Spec.LastBuild = nil
			v1alpha1Build.Spec.Notary = nil
			v1alpha1Build.Spec.CacheName = ""
			require.Equal(t, testV1alpha1Build, v1alpha1Build)

			testV1alpha2Build := &Build{}
			err = testV1alpha2Build.ConvertFrom(context.TODO(), v1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Build, v1alpha2Build)
		})

		it("handles converting builds with a registry cache", func() {
			v1alpha2Build.Spec.Cache = &BuildCacheConfig{
				Registry: &RegistryCache{
					Tag: "registry.com/tag",
				},
			}
			testV1alpha1Build := &v1alpha1.Build{}
			err := v1alpha2Build.ConvertTo(context.TODO(), testV1alpha1Build)
			require.NoError(t, err)
		})

		it("converts v1alpha1 bindings", func() {
			testV1alpha2Build := &Build{}
			bindings := corev1alpha1.CNBBindings{
				{
					Name:        "some-binding",
					MetadataRef: &corev1.LocalObjectReference{Name: "some-metadata"},
					SecretRef:   &corev1.LocalObjectReference{Name: "some-secret"},
				},
			}
			v1alpha1Build.Spec.Bindings = bindings

			err := testV1alpha2Build.ConvertFrom(context.TODO(), v1alpha1Build)
			require.NoError(t, err)
			require.Equal(t, bindings, testV1alpha2Build.CnbBindings())
		})
	})
}
