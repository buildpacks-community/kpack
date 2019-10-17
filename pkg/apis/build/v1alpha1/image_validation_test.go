package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestImageValidation(t *testing.T) {
	spec.Run(t, "Image Validation", testImageValidation)
}

func testImageValidation(t *testing.T, when spec.G, it spec.S) {
	var limit int64 = 90
	image := &Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: ImageSpec{
			Tag: "some/image",
			Builder: ImageBuilder{
				TypeMeta: metav1.TypeMeta{
					Kind: "ClusterBuilder",
				},
				Name: "builder-name",
			},
			ServiceAccount: "some/service-account",
			Source: SourceConfig{
				Git: &Git{
					URL:      "http://github.com/repo",
					Revision: "master",
				},
			},
			FailedBuildHistoryLimit:  &limit,
			SuccessBuildHistoryLimit: &limit,
			ImageTaggingStrategy:     None,
			Build: ImageBuild{
				Env: []v1.EnvVar{
					{
						Name:  "keyA",
						Value: "ValueA",
					},
					{
						Name:  "keyB",
						Value: "ValueB",
					},
				},
			},
		},
	}

	when("Default", func() {
		it("does not modify already set fields", func() {
			oldImage := image.DeepCopy()
			image.SetDefaults(context.TODO())

			assert.Equal(t, image, oldImage)
		})

		it("defaults service account to default", func() {
			image.Spec.ServiceAccount = ""

			image.SetDefaults(context.TODO())

			assert.Equal(t, image.Spec.ServiceAccount, "default")
		})

		it("defaults ImageTaggingStrategy to BuildNumber", func() {
			image.Spec.ImageTaggingStrategy = ""

			image.SetDefaults(context.TODO())

			assert.Equal(t, image.Spec.ImageTaggingStrategy, BuildNumber)
		})

		it("defaults SuccessBuildHistoryLimit,FailedBuildHistoryLimit to 10", func() {
			image.Spec.SuccessBuildHistoryLimit = nil
			image.Spec.FailedBuildHistoryLimit = nil

			image.SetDefaults(context.TODO())

			assert.Equal(t, *image.Spec.SuccessBuildHistoryLimit, int64(10))
			assert.Equal(t, *image.Spec.FailedBuildHistoryLimit, int64(10))
		})

	})

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, image.Validate(context.TODO()))
		})

		assertValidationError := func(image *Image, expectedError *apis.FieldError) {
			t.Helper()
			err := image.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing field tag", func() {
			image.Spec.Tag = ""
			assertValidationError(image, apis.ErrMissingField("tag").ViaField("spec"))
		})

		it("missing builder name", func() {
			image.Spec.Builder.Name = ""
			assertValidationError(image, apis.ErrMissingField("name").ViaField("spec", "builder"))
		})

		it("invalid builder Kind", func() {
			image.Spec.Builder.Kind = "FakeBuilder"
			assertValidationError(image, apis.ErrInvalidValue("FakeBuilder", "kind").ViaField("spec", "builder"))
		})

		it("multiple sources", func() {
			image.Spec.Source.Git = &Git{
				URL:      "http://github.com/repo",
				Revision: "master",
			}
			image.Spec.Source.Blob = &Blob{
				URL: "http://blob.com/url",
			}
			assertValidationError(image, apis.ErrMultipleOneOf("git", "blob").ViaField("spec", "source"))

			image.Spec.Source.Registry = &Registry{
				Image: "registry.com/image",
			}
			assertValidationError(image, apis.ErrMultipleOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("missing source", func() {
			image.Spec.Source = SourceConfig{}

			assertValidationError(image, apis.ErrMissingOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("validates git url", func() {
			image.Spec.Source.Git = &Git{
				URL:      "",
				Revision: "master",
			}

			assertValidationError(image, apis.ErrMissingField("url").ViaField("spec", "source", "git"))
		})

		it("validates git revision", func() {
			image.Spec.Source.Git = &Git{
				URL:      "http://github.com/url",
				Revision: "",
			}

			assertValidationError(image, apis.ErrMissingField("revision").ViaField("spec", "source", "git"))
		})

		it("validates blob url", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Blob = &Blob{URL: ""}

			assertValidationError(image, apis.ErrMissingField("url").ViaField("spec", "source", "blob"))
		})

		it("validates registry url", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Registry = &Registry{Image: ""}

			assertValidationError(image, apis.ErrMissingField("image").ViaField("spec", "source", "registry"))
		})

		it("combining errors", func() {
			image.Spec.Tag = ""
			image.Spec.Builder.Kind = "FakeBuilder"
			assertValidationError(image,
				apis.ErrMissingField("tag").ViaField("spec").
					Also(apis.ErrInvalidValue("FakeBuilder", "kind").ViaField("spec", "builder")))
		})
	})
}
