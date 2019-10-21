package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestBuilderValidation(t *testing.T) {
	spec.Run(t, "Builder Validation", testBuilderValidation)
}

func testBuilderValidation(t *testing.T, when spec.G, it spec.S) {
	builder := &Builder{
		TypeMeta: metav1.TypeMeta{
			Kind: "Builder",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "builder-name",
		},
		Spec: BuilderWithSecretsSpec{
			BuilderSpec: BuilderSpec{
				Image:        "cloudfoundry/cnb:bionic",
				UpdatePolicy: "external",
			},
			ImagePullSecrets: nil,
		},
	}

	clusterBuilder := &ClusterBuilder{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterBuilder",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "builder-name",
		},
		Spec: BuilderSpec{
			Image:        "cloudfoundry/cnb:bionic",
			UpdatePolicy: "external",
		},
	}
	when("Builder", func() {

		when("Default", func() {
			it("does not modify already set fields", func() {
				oldBuilder := builder.DeepCopy()
				builder.SetDefaults(context.TODO())

				assert.Equal(t, builder, oldBuilder)
			})

			it("defaults UpdatePolicy to polling", func() {
				builder.Spec.BuilderSpec.UpdatePolicy = ""

				builder.SetDefaults(context.TODO())

				assert.Equal(t, builder.Spec.BuilderSpec.UpdatePolicy, Polling)
			})
		})

		when("Validate", func() {
			it("returns nil on no validation error", func() {
				assert.Nil(t, builder.Validate(context.TODO()))
			})

			assertValidationError := func(builder *Builder, expectedError *apis.FieldError) {
				t.Helper()
				err := builder.Validate(context.TODO())
				assert.EqualError(t, err, expectedError.Error())
			}

			it("missing field image", func() {
				builder.Spec.Image = ""
				assertValidationError(builder, apis.ErrMissingField("image").ViaField("spec"))
			})

			it("invalid field image", func() {
				builder.Spec.Image = "builderbuilderbuilder@foo"
				assertValidationError(builder, apis.ErrInvalidValue("builderbuilderbuilder@foo", "image").ViaField("spec"))
			})
		})
	})

	when("ClusterBuilder", func() {
		when("Default", func() {
			it("does not modify already set fields", func() {
				oldClusterBuilder := clusterBuilder.DeepCopy()
				clusterBuilder.SetDefaults(context.TODO())

				assert.Equal(t, clusterBuilder, oldClusterBuilder)
			})

			it("defaults UpdatePolicy to polling", func() {
				clusterBuilder.Spec.UpdatePolicy = ""

				clusterBuilder.SetDefaults(context.TODO())

				assert.Equal(t, clusterBuilder.Spec.UpdatePolicy, Polling)
			})
		})

		when("Validate", func() {
			it("returns nil on no validation error", func() {
				assert.Nil(t, clusterBuilder.Validate(context.TODO()))
			})

			assertValidationError := func(builder *Builder, expectedError *apis.FieldError) {
				t.Helper()
				err := clusterBuilder.Validate(context.TODO())
				assert.EqualError(t, err, expectedError.Error())
			}

			it("missing field image", func() {
				clusterBuilder.Spec.Image = ""
				assertValidationError(builder, apis.ErrMissingField("image").ViaField("spec"))
			})
		})
	})
}
