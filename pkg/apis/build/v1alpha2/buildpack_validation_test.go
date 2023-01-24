package v1alpha2

import (
	"context"
	"testing"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestBuildpackValidation(t *testing.T) {
	spec.Run(t, "Buildpack Validation", testBuildpackValidation)
}

func testBuildpackValidation(t *testing.T, when spec.G, it spec.S) {
	buildpack := &Buildpack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-builder-name",
			Namespace: "custom-builder-namespace",
		},
		Spec: BuildpackSpec{
			Source: corev1alpha1.ImageSource{
				Image: "some-registry.io/store-image-1@sha256:78c1b9419976227e05be9d243b7fa583bea44a5258e52018b2af4cdfe23d148d",
			},
			ServiceAccountName: "some-service-account",
		},
	}

	when("Default", func() {
		it("does not modify already set fields", func() {
			oldBuildpack := buildpack.DeepCopy()
			buildpack.SetDefaults(context.TODO())

			assert.Equal(t, buildpack, oldBuildpack)
		})

		it("defaults service account to default", func() {
			buildpack.Spec.ServiceAccountName = ""
			buildpack.SetDefaults(context.TODO())
			assert.Equal(t, buildpack.Spec.ServiceAccountName, "default")
		})
	})

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, buildpack.Validate(context.TODO()))
		})

		assertValidationError := func(buildpack *Buildpack, expectedError *apis.FieldError) {
			t.Helper()
			err := buildpack.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing source image", func() {
			buildpack.Spec.Source.Image = ""
			assertValidationError(buildpack, apis.ErrMissingField("image").ViaField("spec", "source"))
		})

		it("invalid source image", func() {
			buildpack.Spec.Source.Image = "ftp//invalid/tag@@"

			assertValidationError(buildpack,
				apis.ErrInvalidValue(buildpack.Spec.Source.Image, "image").ViaField("spec", "source"),
			)
		})
	})
}
