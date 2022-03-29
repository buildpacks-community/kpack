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

func TestImageConversion(t *testing.T) {
	spec.Run(t, "Test Image Conversion", testImageConversion)
}

func testImageConversion(t *testing.T, when spec.G, it spec.S) {
	when("converting to v1alpha1 and back", func() {
		cacheSize := resource.MustParse("5G")
		var buildHistoryLimit int64 = 5

		v1alpha2Image := &Image{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
				Name:        "my-super-convertable-image",
			},
			Spec: ImageSpec{
				Tag:                "my-tag",
				Builder:            corev1.ObjectReference{},
				ServiceAccountName: "service-account",
				Source: corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      "https://my-github.com/git",
						Revision: "main",
					},
					SubPath: "sub-path",
				},
				Cache: &ImageCacheConfig{
					Volume: &ImagePersistentVolumeCache{
						Size: &cacheSize,
					},
				},
				FailedBuildHistoryLimit:  &buildHistoryLimit,
				SuccessBuildHistoryLimit: &buildHistoryLimit,
				ImageTaggingStrategy:     corev1alpha1.BuildNumber,
				Build: &ImageBuild{
					Services: Services{
						{
							Kind:       "Secret",
							Name:       "some-secret",
							APIVersion: "v1",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "blah",
							Value: "env",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{"some-limit": resource.Quantity{}},
						Requests: corev1.ResourceList{"some-request": resource.Quantity{}},
					},
				},
				Notary: &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "notary.com",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "shhh",
						},
					},
				},
			},
			Status: ImageStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: []corev1alpha1.Condition{{
						Type:               corev1alpha1.ConditionReady,
						Status:             "True",
						Severity:           "tornado-warning",
						LastTransitionTime: corev1alpha1.VolatileTime{},
						Reason:             "executive-order",
						Message:            "it-is-too-late",
					}},
				},
				LatestBuildRef:             "some-build",
				LatestBuildImageGeneration: 1,
				LatestImage:                "my-repo/my-image",
				LatestStack:                "io.buildpacks.stacks.full",
				BuildCounter:               1,
				BuildCacheName:             "build-pvc",
				LatestBuildReason:          "COMMIT",
			},
		}
		v1alpha1Image := &v1alpha1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-super-convertable-image",
				Annotations: map[string]string{
					"kpack.io/services": `[{"kind":"Secret","name":"some-secret","apiVersion":"v1"}]`,
				},
			},
			Spec: v1alpha1.ImageSpec{
				Tag:            "my-tag",
				Builder:        corev1.ObjectReference{},
				ServiceAccount: "service-account",
				Source: corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      "https://my-github.com/git",
						Revision: "main",
					},
					SubPath: "sub-path",
				},
				CacheSize:                &cacheSize,
				FailedBuildHistoryLimit:  &buildHistoryLimit,
				SuccessBuildHistoryLimit: &buildHistoryLimit,
				ImageTaggingStrategy:     corev1alpha1.BuildNumber,
				Build: &v1alpha1.ImageBuild{
					Bindings: nil,
					Env: []corev1.EnvVar{
						{
							Name:  "blah",
							Value: "env",
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits:   corev1.ResourceList{"some-limit": resource.Quantity{}},
						Requests: corev1.ResourceList{"some-request": resource.Quantity{}},
					},
				},
				Notary: &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "notary.com",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "shhh",
						},
					},
				},
			},
			Status: v1alpha1.ImageStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: []corev1alpha1.Condition{{
						Type:               corev1alpha1.ConditionReady,
						Status:             "True",
						Severity:           "tornado-warning",
						LastTransitionTime: corev1alpha1.VolatileTime{},
						Reason:             "executive-order",
						Message:            "it-is-too-late",
					}},
				},
				LatestBuildRef:             "some-build",
				LatestBuildImageGeneration: 1,
				LatestImage:                "my-repo/my-image",
				LatestStack:                "io.buildpacks.stacks.full",
				BuildCounter:               1,
				BuildCacheName:             "build-pvc",
				LatestBuildReason:          "COMMIT",
			},
		}

		it("can convert a git source image", func() {
			v1alpha2Image.Spec.Source = corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "my-repo.com",
					Revision: "main",
				},
				SubPath: "my-sub-path",
			}
			v1alpha1Image.Spec.Source = corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "my-repo.com",
					Revision: "main",
				},
				SubPath: "my-sub-path",
			}
			testV1alpha1Image := &v1alpha1.Image{}
			err := v1alpha2Image.ConvertTo(context.TODO(), testV1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, testV1alpha1Image, v1alpha1Image)

			testV1alpha2Image := &Image{}
			err = testV1alpha2Image.ConvertFrom(context.TODO(), v1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Image, v1alpha2Image)
		})

		it("can convert a registry source image", func() {
			v1alpha2Image.Spec.Source = corev1alpha1.SourceConfig{
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
			v1alpha1Image.Spec.Source = corev1alpha1.SourceConfig{
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

			testV1alpha1Image := &v1alpha1.Image{}
			err := v1alpha2Image.ConvertTo(context.TODO(), testV1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, testV1alpha1Image, v1alpha1Image)

			testV1alpha2Image := &Image{}
			err = testV1alpha2Image.ConvertFrom(context.TODO(), v1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Image, v1alpha2Image)
		})

		it("can convert a blob source image", func() {
			v1alpha2Image.Spec.Source = corev1alpha1.SourceConfig{
				Blob: &corev1alpha1.Blob{
					URL: "blob.com",
				},
				SubPath: "my-sub-path",
			}
			v1alpha1Image.Spec.Source = corev1alpha1.SourceConfig{
				Blob: &corev1alpha1.Blob{
					URL: "blob.com",
				},
				SubPath: "my-sub-path",
			}

			testV1alpha1Image := &v1alpha1.Image{}
			err := v1alpha2Image.ConvertTo(context.TODO(), testV1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, testV1alpha1Image, v1alpha1Image)

			testV1alpha2Image := &Image{}
			err = testV1alpha2Image.ConvertFrom(context.TODO(), v1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Image, v1alpha2Image)
		})

		it("handles nil values", func() {
			v1alpha2Image.Spec.Build = nil
			v1alpha2Image.Spec.Notary = nil
			v1alpha2Image.Spec.Cache = nil

			testV1alpha1Image := &v1alpha1.Image{}
			err := v1alpha2Image.ConvertTo(context.TODO(), testV1alpha1Image)
			require.NoError(t, err)

			v1alpha1Image.Spec.Build = nil
			v1alpha1Image.Spec.Notary = nil
			v1alpha1Image.Spec.CacheSize = nil
			v1alpha1Image.Annotations = map[string]string{}
			require.Equal(t, testV1alpha1Image, v1alpha1Image)

			testV1alpha2Image := &Image{}
			err = testV1alpha2Image.ConvertFrom(context.TODO(), v1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Image, v1alpha2Image)
		})

		it("handles converting images with registry cache", func() {
			v1alpha2Image.Spec.Cache = &ImageCacheConfig{
				Registry: &RegistryCache{
					Tag: "some-registry.com/tag",
				},
			}
			testV1alpha1Image := &v1alpha1.Image{}
			err := v1alpha2Image.ConvertTo(context.TODO(), testV1alpha1Image)
			require.NoError(t, err)
		})

		it("converts v1alpha1 bindings", func() {
			testV1Alpha2Image := &Image{}
			bindings := corev1alpha1.CNBBindings{
				{
					Name:        "some-binding",
					MetadataRef: &corev1.LocalObjectReference{Name: "some-metadata"},
					SecretRef:   &corev1.LocalObjectReference{Name: "some-secret"},
				},
			}
			v1alpha1Image.Spec.Build.Bindings = bindings

			err := testV1Alpha2Image.ConvertFrom(context.TODO(), v1alpha1Image)
			require.NoError(t, err)
			require.Equal(t, bindings, testV1Alpha2Image.CNBBindings())
		})
	})
}
