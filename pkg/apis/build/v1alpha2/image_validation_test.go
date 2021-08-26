package v1alpha2

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
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
	ctx := context.WithValue(context.TODO(), HasDefaultStorageClass, true)
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
				Env: []corev1.EnvVar{
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
			image.SetDefaults(ctx)

			assert.Equal(t, image, oldImage)
		})

		it("defaults service account to default", func() {
			image.Spec.ServiceAccount = ""

			image.SetDefaults(ctx)

			assert.Equal(t, image.Spec.ServiceAccount, "default")
		})

		it("defaults ImageTaggingStrategy to BuildNumber", func() {
			image.Spec.ImageTaggingStrategy = ""

			image.SetDefaults(ctx)

			assert.Equal(t, image.Spec.ImageTaggingStrategy, BuildNumber)
		})

		it("defaults SuccessBuildHistoryLimit,FailedBuildHistoryLimit to 10", func() {
			image.Spec.SuccessBuildHistoryLimit = nil
			image.Spec.FailedBuildHistoryLimit = nil

			image.SetDefaults(ctx)

			assert.Equal(t, *image.Spec.SuccessBuildHistoryLimit, int64(10))
			assert.Equal(t, *image.Spec.FailedBuildHistoryLimit, int64(10))
		})

		when("the cache is not provided", func() {
			image.Spec.CacheSize = nil

			when("the context has the default storage class key", func() {
				it("sets the default cache size", func() {
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
			assert.Nil(t, image.Validate(ctx))

			for _, builderKind := range []string{"Builder", "ClusterBuilder"} {
				image.Spec.Builder.Kind = builderKind
				assert.Nil(t, image.Validate(ctx))
			}
		})

		assertValidationError := func(image *Image, ctx context.Context, expectedError error) {
			t.Helper()
			err := image.Validate(ctx)
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing field tag", func() {
			image.Spec.Tag = ""
			assertValidationError(image, ctx, apis.ErrMissingField("tag").ViaField("spec"))
		})

		it("invalid image tag", func() {
			image.Spec.Tag = "ftp//invalid/tag@@"

			assertValidationError(image, ctx, apis.ErrInvalidValue(image.Spec.Tag, "tag").ViaField("spec"))
		})

		it("tag does not contain fully qualified digest", func() {
			image.Spec.Tag = "some/app@sha256:72d10a33e3233657832967acffce652b729961da5247550ea58b2c2389cddc68"

			assertValidationError(image, ctx, apis.ErrInvalidValue(image.Spec.Tag, "tag").ViaField("spec"))
		})

		it("missing builder name", func() {
			image.Spec.Builder.Name = ""
			assertValidationError(image, ctx, apis.ErrMissingField("name").ViaField("spec", "builder"))
		})

		it("invalid builder Kind", func() {
			image.Spec.Builder.Kind = "FakeBuilder"
			assertValidationError(image, ctx, apis.ErrInvalidValue("FakeBuilder", "kind").ViaField("spec", "builder"))
		})

		it("multiple sources", func() {
			image.Spec.Source.Git = &Git{
				URL:      "http://github.com/repo",
				Revision: "master",
			}
			image.Spec.Source.Blob = &Blob{
				URL: "http://blob.com/url",
			}
			assertValidationError(image, ctx, apis.ErrMultipleOneOf("git", "blob").ViaField("spec", "source"))

			image.Spec.Source.Registry = &Registry{
				Image: "registry.com/image",
			}
			assertValidationError(image, ctx, apis.ErrMultipleOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("missing source", func() {
			image.Spec.Source = SourceConfig{}

			assertValidationError(image, ctx, apis.ErrMissingOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("validates git url", func() {
			image.Spec.Source.Git = &Git{
				URL:      "",
				Revision: "master",
			}

			assertValidationError(image, ctx, apis.ErrMissingField("url").ViaField("spec", "source", "git"))
		})

		it("validates git revision", func() {
			image.Spec.Source.Git = &Git{
				URL:      "http://github.com/url",
				Revision: "",
			}

			assertValidationError(image, ctx, apis.ErrMissingField("revision").ViaField("spec", "source", "git"))
		})

		it("validates blob url", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Blob = &Blob{URL: ""}

			assertValidationError(image, ctx, apis.ErrMissingField("url").ViaField("spec", "source", "blob"))
		})

		it("validates registry image exists", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Registry = &Registry{Image: ""}

			assertValidationError(image, ctx, apis.ErrMissingField("image").ViaField("spec", "source", "registry"))
		})

		it("validates registry image valide", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Registry = &Registry{Image: "NotValid@@!"}

			assertValidationError(image, ctx, apis.ErrInvalidValue(image.Spec.Source.Registry.Image, "image").ViaField("spec", "source", "registry"))
		})

		it("validates build bindings", func() {
			image.Spec.Build.Bindings = []Binding{
				{MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
			}

			assertValidationError(image, ctx, apis.ErrMissingField("spec.build.bindings[0].name"))
		})

		it("image name is too long", func() {
			image.ObjectMeta.Name = "this-image-name-that-is-too-long-some-sha-that-is-long-82cb521d636b282340378d80a6307a08e3d4a4c4"
			assertValidationError(image, ctx, errors.New("invalid image name: this-image-name-that-is-too-long-some-sha-that-is-long-82cb521d636b282340378d80a6307a08e3d4a4c4, name must be a a valid label: metadata.name\nmust be no more than 63 characters"))
		})

		it("invalid image name format", func() {
			image.ObjectMeta.Name = "@NOT!!!VALID!!!"
			errMsg := "invalid image name: @NOT!!!VALID!!!, name must be a a valid label: metadata.name\na valid label must be an empty string or consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue',  or 'my_value',  or '12345', regex used for validation is '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')"

			assertValidationError(image, ctx, errors.New(errMsg))
		})

		it("validates cache size is not set when there is no default StorageClass", func() {
			ctx = context.TODO()

			assertValidationError(image, ctx, apis.ErrGeneric("spec.cacheSize cannot be set with no default StorageClass"))
		})

		it("combining errors", func() {
			image.Spec.Tag = ""
			image.Spec.Builder.Kind = "FakeBuilder"
			assertValidationError(image, ctx,
				apis.ErrMissingField("tag").ViaField("spec").
					Also(apis.ErrInvalidValue("FakeBuilder", "kind").ViaField("spec", "builder")))
		})

		it("image.tag has not changed", func() {
			original := image.DeepCopy()

			image.Spec.Tag = "something/different"
			err := image.Validate(apis.WithinUpdate(ctx, original))
			assert.EqualError(t, err, "Immutable field changed: spec.tag\ngot: something/different, want: some/image")
		})

		it("image.cacheSize has not decreased", func() {
			original := image.DeepCopy()
			cacheSize := resource.MustParse("4G")
			image.Spec.CacheSize = &cacheSize
			err := image.Validate(apis.WithinUpdate(ctx, original))
			assert.EqualError(t, err, "Field cannot be decreased: spec.cacheSize\ncurrent: 5G, requested: 4G")
		})

		when("validating the cosign config", func() {
			// cosign: nil
			it("handles nil cosign", func() {
				image.Spec.Cosign = nil
				assert.Nil(t, image.Validate(ctx))
			})

			// cosign: { annotations: nil }
			it("handles nil annotations", func() {
				image.Spec.Cosign = &CosignConfig{
					Annotations: nil,
				}
				assert.Nil(t, image.Validate(ctx))
			})

			// cosign: { Annotations: [] }
			it("handles empty annotations", func() {
				image.Spec.Cosign = &CosignConfig{
					Annotations: []CosignAnnotation{},
				}
				assert.Nil(t, image.Validate(ctx))
			})

			// cosign: { Annotations: [{name: "1", value: "1"}] }
			it("handles annotations", func() {
				image.Spec.Cosign = &CosignConfig{
					Annotations: []CosignAnnotation{
						{
							Name:  "1",
							Value: "1",
						},
					},
				}
				assert.Nil(t, image.Validate(ctx))
			})

			// cosign: { Annotations: [{ Value: "1"}, { Name: "1"}] }
			it("errors on missing annotation fields", func() {
				image.Spec.Cosign = &CosignConfig{
					Annotations: []CosignAnnotation{
						{Value: "1"},
						{Name: "1"},
					},
				}

				err := image.Validate(ctx)
				assert.EqualError(t, err, "missing field(s): spec.cosign.annotations[0].name, spec.cosign.annotations[1].value")
			})
		})

		when("validating the notary config", func() {
			it("handles a valid notary config", func() {
				image.Spec.Notary = &NotaryConfig{
					V1: &NotaryV1Config{
						URL: "some-url",
						SecretRef: NotarySecretRef{
							Name: "some-secret-name",
						},
					},
				}
				assert.Nil(t, image.Validate(ctx))
			})

			it("handles an empty notary url", func() {
				image.Spec.Notary = &NotaryConfig{
					V1: &NotaryV1Config{
						URL: "",
						SecretRef: NotarySecretRef{
							Name: "some-secret-name",
						},
					},
				}
				err := image.Validate(ctx)
				assert.EqualError(t, err, "missing field(s): spec.notary.v1.url")
			})

			it("handles an empty notary secret ref", func() {
				image.Spec.Notary = &NotaryConfig{
					V1: &NotaryV1Config{
						URL: "some-url",
						SecretRef: NotarySecretRef{
							Name: "",
						},
					},
				}
				err := image.Validate(ctx)
				assert.EqualError(t, err, "missing field(s): spec.notary.v1.secretRef.name")
			})
		})
	})
}
