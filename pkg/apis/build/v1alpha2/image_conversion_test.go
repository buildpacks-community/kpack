package v1alpha2_test

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func TestImageConversion(t *testing.T) {
	spec.Run(t, "TestImageConversion", testImageConversion)
}

func testImageConversion(t *testing.T, when spec.G, it spec.S) {
	when("ConvertTo", func() {
		it("converts services to bindings", func() {
			img := v1alpha2.Image{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
				},
				Spec: v1alpha2.ImageSpec{
					Tag: "some-tag",
					Builder: v1.ObjectReference{
						Name: "some-name",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheSize: &resource.Quantity{
						Format: "2G",
					},
					ImageTaggingStrategy: "some-strat",
					Build: &v1alpha2.ImageBuild{
						Env: []v1.EnvVar{{Name: "some-env"}},
						Services: v1alpha2.Services{
							{Name: "some-service", Kind: "some-kind"},
						},
					},
				},
			}

			expectedV1Img := v1alpha1.Image{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
				},
				Spec: v1alpha1.ImageSpec{
					Tag: "some-tag",
					Builder: v1.ObjectReference{
						Name: "some-name",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheSize: &resource.Quantity{
						Format: "2G",
					},
					ImageTaggingStrategy: "some-strat",
					Build: &v1alpha1.ImageBuild{
						Env: []v1.EnvVar{{Name: "some-env"}},
						Bindings: v1alpha1.Bindings{
							{
								Name: "some-service",
								SecretRef: &v1.LocalObjectReference{
									Name: "some-service",
								},
							},
						},
					},
				},
			}
			var v1Img v1alpha1.Image
			require.NoError(t, img.ConvertTo(context.TODO(), &v1Img))
			require.Equal(t, expectedV1Img, v1Img)
		})

		it("errors with unexpected type", func() {
			b := v1alpha2.Image{}

			require.EqualError(t, b.ConvertTo(context.TODO(), &v1alpha2.Image{}), "unsupported type *v1alpha2.Image")
		})
	})

	when("ConvertFrom", func() {
		it("converts bindings to an annotation", func() {
			v1Img := v1alpha1.Image{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
				},
				Spec: v1alpha1.ImageSpec{
					Tag: "some-tag",
					Builder: v1.ObjectReference{
						Name: "some-name",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheSize: &resource.Quantity{
						Format: "2G",
					},
					ImageTaggingStrategy: "some-strat",
					Build: &v1alpha1.ImageBuild{
						Env: []v1.EnvVar{{Name: "some-env"}},
						Bindings: v1alpha1.Bindings{
							{
								Name: "some-name",
								SecretRef: &v1.LocalObjectReference{
									Name: "some-secret",
								},
								MetadataRef: &v1.LocalObjectReference{
									Name: "some-meta",
								},
							},
						},
					},
				},
			}

			expectedImg := v1alpha2.Image{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
					Annotations: map[string]string{
						v1alpha2.V1Alpha1BindingsAnnotation: `[{"name":"some-name","metadataRef":{"name":"some-meta"},"secretRef":{"name":"some-secret"}}]`,
					},
				},
				Spec: v1alpha2.ImageSpec{
					Tag: "some-tag",
					Builder: v1.ObjectReference{
						Name: "some-name",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheSize: &resource.Quantity{
						Format: "2G",
					},
					ImageTaggingStrategy: "some-strat",
					Build: &v1alpha2.ImageBuild{
						Env: []v1.EnvVar{{Name: "some-env"}},
					},
				},
			}

			var img v1alpha2.Image
			require.NoError(t, img.ConvertFrom(context.TODO(), &v1Img))
			require.Equal(t, expectedImg, img)
		})

		it("errors with unexpected type", func() {
			b := v1alpha2.Image{}

			require.EqualError(t, b.ConvertTo(context.TODO(), &v1alpha2.Image{}), "unsupported type *v1alpha2.Image")
		})
	})
}
