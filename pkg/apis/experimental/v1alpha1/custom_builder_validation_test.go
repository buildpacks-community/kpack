package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestCustomClusterBuilderValidation(t *testing.T) {
	spec.Run(t, "Custom Cluster Builder Validation", testCustomClusterBuilderValidation)
}

func testCustomClusterBuilderValidation(t *testing.T, when spec.G, it spec.S) {
	customBuilder := &CustomBuilder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-builder-name",
			Namespace: "custom-builder-namespace",
		},
		Spec: CustomNamespacedBuilderSpec{
			CustomBuilderSpec: CustomBuilderSpec{
				Tag:   "some-registry.io/custom-builder",
				Stack: "some-stack",
				Store: "some-registry.io/store",
				Order: nil, // No order validation
			},
			ServiceAccount: "some-service-account",
		},
	}

	when("Default", func() {
		it("does not modify already set fields", func() {
			oldBuilder := customBuilder.DeepCopy()
			customBuilder.SetDefaults(context.TODO())

			assert.Equal(t, customBuilder, oldBuilder)
		})

		it("defaults service account to default", func() {
			customBuilder.Spec.ServiceAccount = ""
			customBuilder.SetDefaults(context.TODO())
			assert.Equal(t, customBuilder.Spec.ServiceAccount, "default")
		})
	})

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, customBuilder.Validate(context.TODO()))
		})

		assertValidationError := func(customBuilder *CustomBuilder, expectedError *apis.FieldError) {
			t.Helper()
			err := customBuilder.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing field tag", func() {
			customBuilder.Spec.Tag = ""
			assertValidationError(customBuilder, apis.ErrMissingField("tag").ViaField("spec"))
		})

		it("invalid image tag", func() {
			customBuilder.Spec.Tag = "ftp//invalid/tag@@"

			assertValidationError(customBuilder, apis.ErrInvalidValue(customBuilder.Spec.Tag, "tag").ViaField("spec"))
		})

		it("tag should not contain a fully qualified digest", func() {
			customBuilder.Spec.Tag = "some/app@sha256:72d10a33e3233657832967acffce652b729961da5247550ea58b2c2389cddc68"

			assertValidationError(customBuilder, apis.ErrInvalidValue(customBuilder.Spec.Tag, "tag").ViaField("spec"))
		})

		it("missing field stack", func() {
			customBuilder.Spec.Stack = ""
			assertValidationError(customBuilder, apis.ErrMissingField("stack").ViaField("spec"))
		})

		it("missing field store", func() {
			customBuilder.Spec.Store = ""
			assertValidationError(customBuilder, apis.ErrMissingField("store").ViaField("spec"))
		})
	})
}
