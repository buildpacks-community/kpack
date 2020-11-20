package v1alpha2_test

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func TestBuildConversion(t *testing.T) {
	spec.Run(t, "TestBuildConversion", testBuildConversion)
}

func testBuildConversion(t *testing.T, when spec.G, it spec.S) {
	when("ConvertTo", func() {
		it("converts services to bindings", func() {
			bld := v1alpha2.Build{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
				},
				Spec: v1alpha2.BuildSpec{
					Tags: []string{"some-tag"},
					Builder: v1alpha1.BuildBuilderSpec{
						Image: "some-image",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheName: "some-cache",
					LastBuild: &v1alpha2.LastBuild{
						Image:   "some-image",
						StackId: "some-stack",
					},
					Env: []v1.EnvVar{{Name: "some-env"}},
					Services: v1alpha2.Services{
						{
							Name: "some-service",
							Kind: "some-kind",
						},
					},
				},
			}

			expectedV1Bld := v1alpha1.Build{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
				},
				Spec: v1alpha1.BuildSpec{
					Tags: []string{"some-tag"},
					Builder: v1alpha1.BuildBuilderSpec{
						Image: "some-image",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheName: "some-cache",
					LastBuild: &v1alpha1.LastBuild{
						Image:   "some-image",
						StackId: "some-stack",
					},
					Env:       []v1.EnvVar{{Name: "some-env"}},
					Resources: v1.ResourceRequirements{},
					Bindings: v1alpha1.Bindings{
						{
							Name: "some-service",
							SecretRef: &v1.LocalObjectReference{
								Name: "some-service",
							},
						},
					},
				},
			}
			var v1Bld v1alpha1.Build
			require.NoError(t, bld.ConvertTo(context.TODO(), &v1Bld))
			require.Equal(t, expectedV1Bld, v1Bld)
		})

		it("errors with unexpected type", func() {
			b := v1alpha2.Build{}

			require.EqualError(t, b.ConvertTo(context.TODO(), &v1alpha2.Build{}), "unsupported type *v1alpha2.Build")
		})
	})

	when("ConvertFrom", func() {
		it("converts bindings to an annotation", func() {
			v1Bld := v1alpha1.Build{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
				},
				Spec: v1alpha1.BuildSpec{
					Tags: []string{"some-tag"},
					Builder: v1alpha1.BuildBuilderSpec{
						Image: "some-image",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheName: "some-cache",
					LastBuild: &v1alpha1.LastBuild{
						Image:   "some-image",
						StackId: "some-stack",
					},
					Env:       []v1.EnvVar{{Name: "some-env"}},
					Resources: v1.ResourceRequirements{},
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
			}

			expectedBld := v1alpha2.Build{
				TypeMeta: metav1.TypeMeta{
					Kind: "some-kind",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-name",
					Annotations: map[string]string{
						v1alpha2.V1Alpha1BindingsAnnotation: `[{"name":"some-name","metadataRef":{"name":"some-meta"},"secretRef":{"name":"some-secret"}}]`,
					},
				},
				Spec: v1alpha2.BuildSpec{
					Tags: []string{"some-tag"},
					Builder: v1alpha1.BuildBuilderSpec{
						Image: "some-image",
					},
					ServiceAccount: "some-sa",
					Source: v1alpha1.SourceConfig{
						SubPath: "some-path",
					},
					CacheName: "some-cache",
					LastBuild: &v1alpha2.LastBuild{
						Image:   "some-image",
						StackId: "some-stack",
					},
					Env:       []v1.EnvVar{{Name: "some-env"}},
					Resources: v1.ResourceRequirements{},
				},
			}

			var bld v1alpha2.Build
			require.NoError(t, bld.ConvertFrom(context.TODO(), &v1Bld))
			require.Equal(t, expectedBld, bld)
		})

		it("errors with unexpected type", func() {
			b := v1alpha2.Build{}

			require.EqualError(t, b.ConvertTo(context.TODO(), &v1alpha2.Build{}), "unsupported type *v1alpha2.Build")
		})
	})
}
