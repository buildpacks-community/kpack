package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestImageValidation(t *testing.T) {
	spec.Run(t, "Image Validation", testImageValidation)
}

func testImageValidation(t *testing.T, when spec.G, it spec.S) {
	var limit int64 = 90
	cacheSize := resource.MustParse("5G")
	image := &Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: ImageSpec{
			Tag: "some/image",
			Builder: corev1.ObjectReference{
				Kind: "ClusterBuilder",
				Name: "builder-name",
			},
			ServiceAccount: "some/service-account",
			Source: SourceConfig{
				Git: &Git{
					URL:      "http://github.com/repo",
					Revision: "master",
				},
			},
			CacheSize:                &cacheSize,
			FailedBuildHistoryLimit:  &limit,
			SuccessBuildHistoryLimit: &limit,
			ImageTaggingStrategy:     None,
			Build: &ImageBuild{
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

		when("the cache is not provided", func() {
			image.Spec.CacheSize = nil

			when("the context has the default storage class key", func() {
				it("sets the default cache size", func() {
					ctx := context.TODO()
					ctx = context.WithValue(ctx, HasDefaultStorageClass, true)

					image.SetDefaults(ctx)

					assert.NotNil(t, image.Spec.CacheSize)
					assert.Equal(t, image.Spec.CacheSize.String(), "2G")
				})
			})

			when("the context does not have the default storage class key", func() {
				it("does not set the default cache size", func() {
					image.SetDefaults(context.TODO())

					assert.Nil(t, image.Spec.CacheSize)
				})
			})
		})
	})

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, image.Validate(context.TODO()))

			for _, builderKind := range []string{"Builder", "ClusterBuilder", "CustomBuilder", "CustomClusterBuilder"} {
				image.Spec.Builder.Kind = builderKind
				assert.Nil(t, image.Validate(context.TODO()))
			}
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

		it("invalid image tag", func() {
			image.Spec.Tag = "ftp//invalid/tag@@"

			assertValidationError(image, apis.ErrInvalidValue(image.Spec.Tag, "tag").ViaField("spec"))
		})

		it("tag does not contain fully qualified digest", func() {
			image.Spec.Tag = "some/app@sha256:72d10a33e3233657832967acffce652b729961da5247550ea58b2c2389cddc68"

			assertValidationError(image, apis.ErrInvalidValue(image.Spec.Tag, "tag").ViaField("spec"))
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

		it("validates registry image exists", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Registry = &Registry{Image: ""}

			assertValidationError(image, apis.ErrMissingField("image").ViaField("spec", "source", "registry"))
		})

		it("validates registry image valide", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Registry = &Registry{Image: "NotValid@@!"}

			assertValidationError(image, apis.ErrInvalidValue(image.Spec.Source.Registry.Image, "image").ViaField("spec", "source", "registry"))
		})

		it("validates build bindings", func() {
			image.Spec.Build.Bindings = []Binding{
				{MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
			}

			assertValidationError(image, apis.ErrMissingField("spec.build.bindings[0].name"))
		})

		it("combining errors", func() {
			image.Spec.Tag = ""
			image.Spec.Builder.Kind = "FakeBuilder"
			assertValidationError(image,
				apis.ErrMissingField("tag").ViaField("spec").
					Also(apis.ErrInvalidValue("FakeBuilder", "kind").ViaField("spec", "builder")))
		})

		it("image.tag has not changed", func() {
			original := image.DeepCopy()

			image.Spec.Tag = "something/different"
			err := image.Validate(apis.WithinUpdate(context.TODO(), original))
			assert.EqualError(t, err, "Immutable field changed: spec.tag\ngot: something/different, want: some/image")
		})
	})
}
