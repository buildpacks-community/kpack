package v1alpha2

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestImageValidation(t *testing.T) {
	spec.Run(t, "Image Validation", testImageValidation)
}

func testImageValidation(t *testing.T, when spec.G, it spec.S) {
	var limit int64 = 90
	cacheSize := resource.MustParse("5G")
	ctx := context.WithValue(context.WithValue(context.TODO(), HasDefaultStorageClass, true), IsExpandable, true)
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
			ServiceAccountName: "some/service-account",
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "http://github.com/repo",
					Revision: "master",
				},
			},
			Cache: &ImageCacheConfig{
				Volume: &ImagePersistentVolumeCache{
					Size: &cacheSize,
				},
			},
			FailedBuildHistoryLimit:  &limit,
			SuccessBuildHistoryLimit: &limit,
			ImageTaggingStrategy:     corev1alpha1.None,
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
			image.Spec.ServiceAccountName = ""

			image.SetDefaults(ctx)

			assert.Equal(t, image.Spec.ServiceAccountName, "default")
		})

		it("defaults ImageTaggingStrategy to BuildNumber", func() {
			image.Spec.ImageTaggingStrategy = ""

			image.SetDefaults(ctx)

			assert.Equal(t, image.Spec.ImageTaggingStrategy, corev1alpha1.BuildNumber)
		})

		it("defaults SuccessBuildHistoryLimit,FailedBuildHistoryLimit to 10", func() {
			image.Spec.SuccessBuildHistoryLimit = nil
			image.Spec.FailedBuildHistoryLimit = nil

			image.SetDefaults(ctx)

			assert.Equal(t, *image.Spec.SuccessBuildHistoryLimit, int64(10))
			assert.Equal(t, *image.Spec.FailedBuildHistoryLimit, int64(10))
		})

		when("the cache is not provided", func() {
			image.Spec.Cache = nil

			when("the context has the default storage class key", func() {
				it("sets the default cache size", func() {
					image.SetDefaults(ctx)

					assert.NotNil(t, image.Spec.Cache.Volume.Size)
					assert.Equal(t, image.Spec.Cache.Volume.Size.String(), "2G")
				})
			})

			when("the context does not have the default storage class key", func() {
				it("does not set the default cache size", func() {
					image.SetDefaults(context.TODO())

					assert.Nil(t, image.Spec.Cache)
				})
			})
		})

		when("registry cache is provided", func() {
			image.Spec.Cache = &ImageCacheConfig{
				Registry: &RegistryCache{
					Tag: "test",
				},
			}
			it("does not default volume cache", func() {
				image.SetDefaults(context.TODO())

				assert.Nil(t, image.Spec.Cache.Volume)
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

		it("invalid additional image tags", func() {
			image.Spec.AdditionalTags = []string{"valid/tag", "invalid/tag@sha256:thisisatag", "also/invalid@@"}
			assertValidationError(image,
				ctx,
				apis.ErrInvalidArrayValue(image.Spec.AdditionalTags[1], "additionalTags", 1).
					Also(apis.ErrInvalidArrayValue(image.Spec.AdditionalTags[2], "additionalTags", 2)).
					ViaField("spec"))
		})

		it("tags from multiple registries", func() {
			image.Spec.AdditionalTags = []string{"valid/tag", "gcr.io/valid/tag"}
			assertValidationError(image, ctx, errors.New("all additionalTags must have the same registry as tag: spec.additionalTags\nexpected registry: index.docker.io, got: gcr.io"))
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
			image.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "http://github.com/repo",
				Revision: "master",
			}
			image.Spec.Source.Blob = &corev1alpha1.Blob{
				URL: "http://blob.com/url",
			}
			assertValidationError(image, ctx, apis.ErrMultipleOneOf("git", "blob").ViaField("spec", "source"))

			image.Spec.Source.Registry = &corev1alpha1.Registry{
				Image: "registry.com/image",
			}
			assertValidationError(image, ctx, apis.ErrMultipleOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("missing source", func() {
			image.Spec.Source = corev1alpha1.SourceConfig{}

			assertValidationError(image, ctx, apis.ErrMissingOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("validates git url", func() {
			image.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "",
				Revision: "master",
			}

			assertValidationError(image, ctx, apis.ErrMissingField("url").ViaField("spec", "source", "git"))
		})

		it("validates git revision", func() {
			image.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "http://github.com/url",
				Revision: "",
			}

			assertValidationError(image, ctx, apis.ErrMissingField("revision").ViaField("spec", "source", "git"))
		})

		it("validates blob url", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Blob = &corev1alpha1.Blob{URL: ""}

			assertValidationError(image, ctx, apis.ErrMissingField("url").ViaField("spec", "source", "blob"))
		})

		it("validates registry image exists", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Registry = &corev1alpha1.Registry{Image: ""}

			assertValidationError(image, ctx, apis.ErrMissingField("image").ViaField("spec", "source", "registry"))
		})

		it("validates registry image is a valid image", func() {
			image.Spec.Source.Git = nil
			image.Spec.Source.Registry = &corev1alpha1.Registry{Image: "NotValid@@!"}

			assertValidationError(image, ctx, apis.ErrInvalidValue(image.Spec.Source.Registry.Image, "image").ViaField("spec", "source", "registry"))
		})

		it("validates service bindings", func() {
			image.Spec.Build.Services = Services{
				{Kind: "Secret"},
			}

			assertValidationError(image, ctx, apis.ErrMissingField("spec.build.services[0].name"))
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

		it("buildHistoryLimit is less than 1", func() {
			errMsg := "build history limit must be greater than 0: spec.%s"
			invalidLimit := int64(0)

			image.Spec.FailedBuildHistoryLimit = &invalidLimit
			assertValidationError(image, ctx, errors.New(fmt.Sprintf(errMsg, "failedBuildHistoryLimit")))

			image.Spec.FailedBuildHistoryLimit = &defaultFailedBuildHistoryLimit
			image.Spec.SuccessBuildHistoryLimit = &invalidLimit
			assertValidationError(image, ctx, errors.New(fmt.Sprintf(errMsg, "successBuildHistoryLimit")))

		})

		it("validates cache size is not set when there is no default StorageClass", func() {
			ctx = context.TODO()

			assertValidationError(image, ctx, apis.ErrGeneric("spec.cache.volume.size cannot be set with no default StorageClass"))
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
			image.Spec.Cache.Volume.Size = &cacheSize
			err := image.Validate(apis.WithinUpdate(ctx, original))
			assert.EqualError(t, err, "Field cannot be decreased: spec.cache.volume.size\ncurrent: 5G, requested: 4G")
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

		it("image.cacheSize has not changed when storageclass is not expandable", func() {
			original := image.DeepCopy()
			cacheSize := resource.MustParse("6G")
			image.Spec.Cache.Volume.Size = &cacheSize
			err := image.Validate(apis.WithinUpdate(context.WithValue(ctx, IsExpandable, false), original))
			assert.EqualError(t, err, "Field cannot be changed, default storage class is not expandable: spec.cache.volume.size\ncurrent: 5G, requested: 6G")
		})

		it("image.cacheSize has changed when storageclass is expandable", func() {
			original := image.DeepCopy()
			cacheSize := resource.MustParse("6G")
			image.Spec.Cache.Volume.Size = &cacheSize
			err := image.Validate(apis.WithinUpdate(ctx, original))
			assert.Nil(t, err)
		})

		it("handles nil cache", func() {
			image.Spec.Cache = nil
			assert.Nil(t, image.Validate(ctx))
		})

		it("validates notary config has not been set by user", func() {
			image.Spec.Notary = &corev1alpha1.NotaryConfig{
				V1: &corev1alpha1.NotaryV1Config{
					URL: "some-url",
					SecretRef: corev1alpha1.NotarySecretRef{
						Name: "some-secret-name",
					},
				},
			}
			err := image.Validate(ctx)
			assert.EqualError(t, err, "use of this field has been deprecated in v1alpha2, please use v1alpha1 for notary image signing: spec.notary")
		})

		when("validating notary if build is created by kpack controller", func() {
			ctx := apis.WithUserInfo(ctx, &authv1.UserInfo{Username: kpackControllerServiceAccountUsername})
			it("handles an empty notary url", func() {
				image.Spec.Notary = &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "some-secret-name",
						},
					},
				}
				err := image.Validate(ctx)
				assert.EqualError(t, err, "missing field(s): spec.notary.v1.url")
			})

			it("handles an empty notary secret ref", func() {
				image.Spec.Notary = &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "some-url",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "",
						},
					},
				}
				err := image.Validate(ctx)
				assert.EqualError(t, err, "missing field(s): spec.notary.v1.secretRef.name")
			})
		})

		it("validates cnb bindings have not been created by a user", func() {
			image.Spec.Build.CNBBindings = []corev1alpha1.CNBBinding{
				{MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
			}

			assertValidationError(image, ctx, apis.ErrGeneric("use of this field has been deprecated in v1alpha2, please use v1alpha1 for CNB bindings", "spec.build.cnbBindings"))

		})

		when("validating cnb bindings if they have been created by the kpack controller", func() {
			ctx := apis.WithUserInfo(ctx, &authv1.UserInfo{Username: kpackControllerServiceAccountUsername})
			it("validates cnb bindings have a name", func() {
				image.Spec.Build.CNBBindings = []corev1alpha1.CNBBinding{
					{MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
				}

				assertValidationError(image, ctx, apis.ErrMissingField("spec.build.cnbBindings[0].name"))
			})

			it("validates cnb bindings have a valid name", func() {
				image.Spec.Build.CNBBindings = []corev1alpha1.CNBBinding{
					{Name: "&", MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
				}

				assertValidationError(image, ctx, apis.ErrInvalidValue("&", "spec.build.cnbBindings[0].name"))
			})

			it("validates cnb bindings have metadata", func() {
				image.Spec.Build.CNBBindings = []corev1alpha1.CNBBinding{
					{Name: "apm"},
				}

				assertValidationError(image, ctx, apis.ErrMissingField("spec.build.cnbBindings[0].metadataRef"))
			})

			it("validates cnb bindings have non-empty metadata", func() {
				image.Spec.Build.CNBBindings = []corev1alpha1.CNBBinding{
					{Name: "apm", MetadataRef: &corev1.LocalObjectReference{}},
				}

				assertValidationError(image, ctx, apis.ErrMissingField("spec.build.cnbBindings[0].metadataRef.name"))
			})

			it("validates cnb bindings have non-empty secrets", func() {
				image.Spec.Build.CNBBindings = []corev1alpha1.CNBBinding{
					{
						Name:        "apm",
						MetadataRef: &corev1.LocalObjectReference{Name: "metadata"},
						SecretRef:   &corev1.LocalObjectReference{},
					},
				}

				assertValidationError(image, ctx, apis.ErrMissingField("spec.build.cnbBindings[0].secretRef.name"))
			})

			it("validates cnb bindings name uniqueness", func() {
				image.Spec.Build.CNBBindings = []corev1alpha1.CNBBinding{
					{
						Name:        "apm",
						MetadataRef: &corev1.LocalObjectReference{Name: "metadata"},
					},
					{
						Name:        "not-apm",
						MetadataRef: &corev1.LocalObjectReference{Name: "metadata"},
						SecretRef:   &corev1.LocalObjectReference{Name: "secret"},
					},
					{
						Name:        "apm",
						MetadataRef: &corev1.LocalObjectReference{Name: "metadata"},
					},
				}

				assertValidationError(image, ctx, apis.ErrGeneric("duplicate binding name \"apm\"", "spec.build.cnbBindings[0].name", "spec.build.cnbBindings[2].name"))
			})
		})

		it("validates not registry AND volume cache are both specified", func() {
			original := image.DeepCopy()

			image.Spec.Cache.Registry = &RegistryCache{Tag: "test"}

			err := image.Validate(apis.WithinUpdate(ctx, original))
			assert.EqualError(t, err, "only one type of cache can be specified: spec.cache.registry, spec.cache.volume")
		})

		it("validates kubernetes.io/os node selector is unset", func() {
			image.Spec.Build.NodeSelector = map[string]string{k8sOSLabel: "some-os"}
			assertValidationError(image, ctx, apis.ErrInvalidKeyName(k8sOSLabel, "spec.build.nodeSelector", "os is determined automatically"))
		})
	})
}
