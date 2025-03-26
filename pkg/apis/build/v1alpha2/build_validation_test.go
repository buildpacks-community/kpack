package v1alpha2

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestBuildValidation(t *testing.T) {
	spec.Run(t, "Build Validation", testBuildValidation)
}

func testBuildValidation(t *testing.T, when spec.G, it spec.S) {
	buildWithImageRef := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "buildWithImageRef-name",
		},
		Spec: BuildSpec{
			Tags: []string{"some/image"},
			Builder: corev1alpha1.BuildBuilderSpec{
				Image:            "builder/bionic-builder@sha256:e431a4f94fb84854fd081da62762192f36fd093fdfb85ad3bc009b9309524e2d",
				ImagePullSecrets: nil,
			},
			ServiceAccountName: "some/service-account",
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "http://github.com/repo",
					Revision: "master",
				},
			},
		},
	}
	buildWithBuilderRef := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "buildWithImageRef-name",
		},
		Spec: BuildSpec{
			Tags: []string{"some/image"},
			Builder: corev1alpha1.BuildBuilderSpec{
				BuilderRef: &corev1.ObjectReference{
					Name: "default",
					Kind: "ClusterBuilder",
				},
			},
			ServiceAccountName: "some/service-account",
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "http://github.com/repo",
					Revision: "master",
				},
			},
		},
	}
	when("Default", func() {
		it("does not modify already set fields", func() {
			oldBuild := buildWithImageRef.DeepCopy()
			buildWithImageRef.SetDefaults(context.TODO())

			assert.Equal(t, buildWithImageRef, oldBuild)
		})

		it("defaults service account to default", func() {
			buildWithImageRef.Spec.ServiceAccountName = ""

			buildWithImageRef.SetDefaults(context.TODO())

			assert.Equal(t, buildWithImageRef.Spec.ServiceAccountName, "default")
		})
	})

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, buildWithImageRef.Validate(context.TODO()))
		})

		assertValidationError := func(build *Build, ctx context.Context, expectedError *apis.FieldError) {
			t.Helper()
			err := build.Validate(ctx)
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing field tag", func() {
			buildWithImageRef.Spec.Tags = []string{}
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingField("tags").ViaField("spec"))
		})

		it("all tags are valid", func() {
			buildWithImageRef.Spec.Tags = []string{"valid/tag", "invalid/tag@sha256:thisisatag", "also/invalid@@"}
			assertValidationError(buildWithImageRef, context.TODO(),
				apis.ErrInvalidArrayValue("invalid/tag@sha256:thisisatag", "tags", 1).
					Also(apis.ErrInvalidArrayValue("also/invalid@@", "tags", 2)).
					ViaField("spec"))
		})

		it("missing builder name ", func() {
			buildWithBuilderRef.Spec.Builder.BuilderRef.Name = ""
			assertValidationError(buildWithBuilderRef, context.TODO(), apis.ErrMissingField("name").ViaField("spec", "builder", "builderRef"))
		})

		it("missing builder kind ", func() {
			buildWithBuilderRef.Spec.Builder.BuilderRef.Kind = ""
			assertValidationError(buildWithBuilderRef, context.TODO(), apis.ErrMissingField("kind").ViaField("spec", "builder", "builderRef"))
		})

		it("image and builderRef are both set", func() {
			buildWithImageRef.Spec.Builder.BuilderRef = &corev1.ObjectReference{Name: "test", Kind: "builder"}
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrGeneric("image and builderRef fields must be mutually exclusive", "image", "builderRef").ViaField("spec", "builder"))
		})

		it("image and builderRef are empty", func() {
			buildWithImageRef.Spec.Builder.Image = ""
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrGeneric("one of the image or builderRef fields must not be empty", "image", "builderRef").ViaField("spec", "builder"))
		})

		it("invalid Image name", func() {
			buildWithImageRef.Spec.Builder.Image = "foo.ioo/builder-but-not-a-builder@sha256:alksdifhjalsouidfh"
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrInvalidValue("foo.ioo/builder-but-not-a-builder@sha256:alksdifhjalsouidfh", "image").ViaField("spec", "builder"))
		})

		it("multiple sources", func() {
			buildWithImageRef.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "http://github.com/repo",
				Revision: "master",
			}
			buildWithImageRef.Spec.Source.Blob = &corev1alpha1.Blob{
				URL: "http://blob.com/url",
			}
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMultipleOneOf("git", "blob").ViaField("spec", "source"))

			buildWithImageRef.Spec.Source.Registry = &corev1alpha1.Registry{
				Image: "registry.com/image",
			}
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMultipleOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("missing source", func() {
			buildWithImageRef.Spec.Source = corev1alpha1.SourceConfig{}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("validates git url", func() {
			buildWithImageRef.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "",
				Revision: "master",
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingField("url").ViaField("spec", "source", "git"))
		})

		it("validates git revision", func() {
			buildWithImageRef.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "http://github.com/url",
				Revision: "",
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingField("revision").ViaField("spec", "source", "git"))
		})

		it("validates blob url", func() {
			buildWithImageRef.Spec.Source.Git = nil
			buildWithImageRef.Spec.Source.Blob = &corev1alpha1.Blob{URL: ""}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingField("url").ViaField("spec", "source", "blob"))
		})

		it("validates registry url", func() {
			buildWithImageRef.Spec.Source.Git = nil
			buildWithImageRef.Spec.Source.Registry = &corev1alpha1.Registry{Image: ""}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingField("image").ViaField("spec", "source", "registry"))
		})

		it("validates valid lastBuilt Image", func() {
			buildWithImageRef.Spec.LastBuild = &LastBuild{Image: "invalid@@"}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrInvalidValue(buildWithImageRef.Spec.LastBuild.Image, "image").ViaField("spec", "lastBuild"))
		})

		it("validates service bindings have a name", func() {
			buildWithImageRef.Spec.Services = []corev1.ObjectReference{
				{
					Kind: "Secret",
				},
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingField("spec.services[0].name"))
		})

		it("validates service bindings have a valid name", func() {
			buildWithImageRef.Spec.Services = []corev1.ObjectReference{
				{
					Kind: "Secret",
					Name: "&",
				},
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrInvalidValue("&", "spec.services[0].name"))
		})

		it("validates service bindings have a kind", func() {
			buildWithImageRef.Spec.Services = []corev1.ObjectReference{
				{
					Name: "my-ref",
				},
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrMissingField("spec.services[0].kind"))
		})

		it("validates service bindings name uniqueness", func() {
			buildWithImageRef.Spec.Services = []corev1.ObjectReference{
				{
					Kind: "Secret",
					Name: "apm",
				},
				{
					Kind: "Secret",
					Name: "not-apm",
				},
				{
					Kind: "Secret",
					Name: "apm",
				},
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrGeneric("duplicate service name \"apm\"", "spec.services[0].name", "spec.services[2].name"))
		})

		it("validates cnb bindings have not been created by a user", func() {
			buildWithImageRef.Spec.CNBBindings = []corev1alpha1.CNBBinding{
				{MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrGeneric("use of this field has been deprecated in v1alpha2, please use v1alpha1 for CNB bindings", "spec.cnbBindings"))

		})

		when("validating cnb bindings if they have been created by the kpack controller", func() {
			ctx := apis.WithUserInfo(context.TODO(), &authv1.UserInfo{Username: kpackControllerServiceAccountUsername})
			it("validates cnb bindings have a name", func() {
				buildWithImageRef.Spec.CNBBindings = []corev1alpha1.CNBBinding{
					{MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
				}

				assertValidationError(buildWithImageRef, ctx, apis.ErrMissingField("spec.cnbBindings[0].name"))
			})

			it("validates cnb bindings have a valid name", func() {
				buildWithImageRef.Spec.CNBBindings = []corev1alpha1.CNBBinding{
					{Name: "&", MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
				}

				assertValidationError(buildWithImageRef, ctx, apis.ErrInvalidValue("&", "spec.cnbBindings[0].name"))
			})

			it("validates cnb bindings have metadata", func() {
				buildWithImageRef.Spec.CNBBindings = []corev1alpha1.CNBBinding{
					{Name: "apm"},
				}

				assertValidationError(buildWithImageRef, ctx, apis.ErrMissingField("spec.cnbBindings[0].metadataRef"))
			})

			it("validates cnb bindings have non-empty metadata", func() {
				buildWithImageRef.Spec.CNBBindings = []corev1alpha1.CNBBinding{
					{Name: "apm", MetadataRef: &corev1.LocalObjectReference{}},
				}

				assertValidationError(buildWithImageRef, ctx, apis.ErrMissingField("spec.cnbBindings[0].metadataRef.name"))
			})

			it("validates cnb bindings have non-empty secrets", func() {
				buildWithImageRef.Spec.CNBBindings = []corev1alpha1.CNBBinding{
					{
						Name:        "apm",
						MetadataRef: &corev1.LocalObjectReference{Name: "metadata"},
						SecretRef:   &corev1.LocalObjectReference{},
					},
				}

				assertValidationError(buildWithImageRef, ctx, apis.ErrMissingField("spec.cnbBindings[0].secretRef.name"))
			})

			it("validates cnb bindings name uniqueness", func() {
				buildWithImageRef.Spec.CNBBindings = []corev1alpha1.CNBBinding{
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

				assertValidationError(buildWithImageRef, ctx, apis.ErrGeneric("duplicate binding name \"apm\"", "spec.cnbBindings[0].name", "spec.cnbBindings[2].name"))
			})
		})

		it("validates notary config has not been set by user", func() {
			buildWithImageRef.Spec.Notary = &corev1alpha1.NotaryConfig{
				V1: &corev1alpha1.NotaryV1Config{
					URL: "",
					SecretRef: corev1alpha1.NotarySecretRef{
						Name: "some-secret-name",
					},
				},
			}
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrGeneric("use of this field has been deprecated in v1alpha2, please use v1alpha1 for notary image signing", "spec.notary"))

		})

		when("validating notary if buildWithImageRef is created by kpack controller", func() {
			ctx := apis.WithUserInfo(context.TODO(), &authv1.UserInfo{Username: kpackControllerServiceAccountUsername})
			it("handles an empty notary url", func() {
				buildWithImageRef.Spec.Notary = &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "some-secret-name",
						},
					},
				}
				err := buildWithImageRef.Validate(ctx)
				assert.EqualError(t, err, "missing field(s): spec.notary.v1.url")
			})

			it("handles an empty notary secret ref", func() {
				buildWithImageRef.Spec.Notary = &corev1alpha1.NotaryConfig{
					V1: &corev1alpha1.NotaryV1Config{
						URL: "some-url",
						SecretRef: corev1alpha1.NotarySecretRef{
							Name: "",
						},
					},
				}
				err := buildWithImageRef.Validate(ctx)
				assert.EqualError(t, err, "missing field(s): spec.notary.v1.secretRef.name")
			})
		})

		it("validates not registry AND volume cache are both specified", func() {
			buildWithImageRef.Spec.Cache = &BuildCacheConfig{
				Volume:   &BuildPersistentVolumeCache{ClaimName: "pvc"},
				Registry: &RegistryCache{Tag: "test"},
			}

			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrGeneric("only one type of cache can be specified", "spec.cache.volume", "spec.cache.registry"))
		})

		it("combining errors", func() {
			buildWithImageRef.Spec.Tags = []string{}
			buildWithImageRef.Spec.Builder.Image = ""
			assertValidationError(buildWithImageRef, context.TODO(),
				apis.ErrMissingField("tags").ViaField("spec").
					Also(apis.ErrGeneric("one of the image or builderRef fields must not be empty", "image", "builderRef").ViaField("spec", "builder")))
		})

		it("validates spec is immutable", func() {
			original := buildWithImageRef.DeepCopy()

			buildWithImageRef.Spec.Source.Git.URL = "http://something/different"
			err := buildWithImageRef.Validate(apis.WithinUpdate(context.TODO(), original))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "http://something/different")

		})

		it("validates kubernetes.io/os node selector is unset", func() {
			buildWithImageRef.Spec.NodeSelector = map[string]string{k8sOSLabel: "some-os"}
			assertValidationError(buildWithImageRef, context.TODO(), apis.ErrInvalidKeyName(k8sOSLabel, "spec.nodeSelector", "os is determined automatically"))
		})
	})
}
