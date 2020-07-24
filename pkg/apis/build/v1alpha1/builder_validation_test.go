package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestBuilderValidation(t *testing.T) {
	spec.Run(t, "Builder Validation", testBuilderValidation)
}

func testBuilderValidation(t *testing.T, when spec.G, it spec.S) {
	builder := &Builder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-builder-name",
			Namespace: "custom-builder-namespace",
		},
		Spec: NamespacedBuilderSpec{
			BuilderSpec: BuilderSpec{
				Tag: "some-registry.io/custom-builder",
				Stack: corev1.ObjectReference{
					Kind: "ClusterStack",
					Name: "some-stack",
				},
				Store: corev1.ObjectReference{
					Kind: "ClusterStore",
					Name: "some-registry.io/store",
				},
				Order: nil, // No order validation
			},
			ServiceAccount: "some-service-account",
		},
	}

	when("Default", func() {
		it("does not modify already set fields", func() {
			oldBuilder := builder.DeepCopy()
			builder.SetDefaults(context.TODO())

			assert.Equal(t, builder, oldBuilder)
		})

		it("defaults service account to default", func() {
			builder.Spec.ServiceAccount = ""
			builder.SetDefaults(context.TODO())
			assert.Equal(t, builder.Spec.ServiceAccount, "default")
		})

		it("defaults stack.kind to ClusterStack", func() {
			builder.Spec.Stack.Kind = ""
			builder.SetDefaults(context.TODO())
			assert.Equal(t, builder.Spec.Stack.Kind, "ClusterStack")
		})

		it("defaults store.kind to ClusterStore", func() {
			builder.Spec.Store.Kind = ""
			builder.SetDefaults(context.TODO())
			assert.Equal(t, builder.Spec.Store.Kind, "ClusterStore")
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

		it("missing field tag", func() {
			builder.Spec.Tag = ""
			assertValidationError(builder, apis.ErrMissingField("tag").ViaField("spec"))
		})

		it("invalid image tag", func() {
			builder.Spec.Tag = "ftp//invalid/tag@@"

			assertValidationError(builder, apis.ErrInvalidValue(builder.Spec.Tag, "tag").ViaField("spec"))
		})

		it("tag should not contain a fully qualified digest", func() {
			builder.Spec.Tag = "some/app@sha256:72d10a33e3233657832967acffce652b729961da5247550ea58b2c2389cddc68"

			assertValidationError(builder, apis.ErrInvalidValue(builder.Spec.Tag, "tag").ViaField("spec"))
		})

		it("missing stack name", func() {
			builder.Spec.Stack.Name = ""
			assertValidationError(builder, apis.ErrMissingField("name").ViaField("spec", "stack"))
		})

		it("invalid stack kind", func() {
			builder.Spec.Stack.Kind = "FakeStack"
			assertValidationError(builder, apis.ErrInvalidValue("FakeStack", "kind").ViaField("spec", "stack"))
		})

		it("missing store name", func() {
			builder.Spec.Store.Name = ""
			assertValidationError(builder, apis.ErrMissingField("name").ViaField("spec", "store"))
		})

		it("invalid store kind", func() {
			builder.Spec.Store.Kind = "FakeStore"
			assertValidationError(builder, apis.ErrInvalidValue("FakeStore", "kind").ViaField("spec", "store"))
		})
	})
}
