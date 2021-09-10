package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestBuildValidation(t *testing.T) {
	spec.Run(t, "Build Validation", testBuildValidation)
}

func testBuildValidation(t *testing.T, when spec.G, it spec.S) {
	build := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "build-name",
		},
		Spec: BuildSpec{
			Tags: []string{"some/image"},
			Builder: corev1alpha1.BuildBuilderSpec{
				Image:            "builder/bionic-builder@sha256:e431a4f94fb84854fd081da62762192f36fd093fdfb85ad3bc009b9309524e2d",
				ImagePullSecrets: nil,
			},
			ServiceAccount: "some/service-account",
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
			oldBuild := build.DeepCopy()
			build.SetDefaults(context.TODO())

			assert.Equal(t, build, oldBuild)
		})

		it("defaults service account to default", func() {
			build.Spec.ServiceAccount = ""

			build.SetDefaults(context.TODO())

			assert.Equal(t, build.Spec.ServiceAccount, "default")
		})
	})

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, build.Validate(context.TODO()))
		})

		assertValidationError := func(build *Build, expectedError *apis.FieldError) {
			t.Helper()
			err := build.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing field tag", func() {
			build.Spec.Tags = []string{}
			assertValidationError(build, apis.ErrMissingField("tags").ViaField("spec"))
		})

		it("all tags are valid", func() {
			build.Spec.Tags = []string{"valid/tag", "invalid/tag@sha256:thisisatag", "also/invalid@@"}
			assertValidationError(build,
				apis.ErrInvalidArrayValue("invalid/tag@sha256:thisisatag", "tags", 1).
					Also(apis.ErrInvalidArrayValue("also/invalid@@", "tags", 2)).
					ViaField("spec"))
		})

		it("missing builder name", func() {
			build.Spec.Builder.Image = ""
			assertValidationError(build, apis.ErrMissingField("image").ViaField("spec", "builder"))
		})

		it("invalid builder name", func() {
			build.Spec.Builder.Image = "foo.ioo/builder-but-not-a-builder@sha256:alksdifhjalsouidfh"
			assertValidationError(build, apis.ErrInvalidValue("foo.ioo/builder-but-not-a-builder@sha256:alksdifhjalsouidfh", "image").ViaField("spec", "builder"))
		})

		it("multiple sources", func() {
			build.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "http://github.com/repo",
				Revision: "master",
			}
			build.Spec.Source.Blob = &corev1alpha1.Blob{
				URL: "http://blob.com/url",
			}
			assertValidationError(build, apis.ErrMultipleOneOf("git", "blob").ViaField("spec", "source"))

			build.Spec.Source.Registry = &corev1alpha1.Registry{
				Image: "registry.com/image",
			}
			assertValidationError(build, apis.ErrMultipleOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("missing source", func() {
			build.Spec.Source = corev1alpha1.SourceConfig{}

			assertValidationError(build, apis.ErrMissingOneOf("git", "blob", "registry").ViaField("spec", "source"))
		})

		it("validates git url", func() {
			build.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "",
				Revision: "master",
			}

			assertValidationError(build, apis.ErrMissingField("url").ViaField("spec", "source", "git"))
		})

		it("validates git revision", func() {
			build.Spec.Source.Git = &corev1alpha1.Git{
				URL:      "http://github.com/url",
				Revision: "",
			}

			assertValidationError(build, apis.ErrMissingField("revision").ViaField("spec", "source", "git"))
		})

		it("validates blob url", func() {
			build.Spec.Source.Git = nil
			build.Spec.Source.Blob = &corev1alpha1.Blob{URL: ""}

			assertValidationError(build, apis.ErrMissingField("url").ViaField("spec", "source", "blob"))
		})

		it("validates registry url", func() {
			build.Spec.Source.Git = nil
			build.Spec.Source.Registry = &corev1alpha1.Registry{Image: ""}

			assertValidationError(build, apis.ErrMissingField("image").ViaField("spec", "source", "registry"))
		})

		it("validates valid lastBuilt Image", func() {
			build.Spec.LastBuild = &LastBuild{Image: "invalid@@"}

			assertValidationError(build, apis.ErrInvalidValue(build.Spec.LastBuild.Image, "image").ViaField("spec", "lastBuild"))
		})

		it("validates bindings have a name", func() {
			build.Spec.Bindings = []corev1alpha1.CNBBinding{
				{MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
			}

			assertValidationError(build, apis.ErrMissingField("spec.bindings[0].name"))
		})

		it("validates bindings have a valid name", func() {
			build.Spec.Bindings = []corev1alpha1.CNBBinding{
				{Name: "&", MetadataRef: &corev1.LocalObjectReference{Name: "metadata"}},
			}

			assertValidationError(build, apis.ErrInvalidValue("&", "spec.bindings[0].name"))
		})

		it("validates bindings have metadata", func() {
			build.Spec.Bindings = []corev1alpha1.CNBBinding{
				{Name: "apm"},
			}

			assertValidationError(build, apis.ErrMissingField("spec.bindings[0].metadataRef"))
		})

		it("validates bindings have non-empty metadata", func() {
			build.Spec.Bindings = []corev1alpha1.CNBBinding{
				{Name: "apm", MetadataRef: &corev1.LocalObjectReference{}},
			}

			assertValidationError(build, apis.ErrMissingField("spec.bindings[0].metadataRef.name"))
		})

		it("validates bindings have non-empty secrets", func() {
			build.Spec.Bindings = []corev1alpha1.CNBBinding{
				{
					Name:        "apm",
					MetadataRef: &corev1.LocalObjectReference{Name: "metadata"},
					SecretRef:   &corev1.LocalObjectReference{},
				},
			}

			assertValidationError(build, apis.ErrMissingField("spec.bindings[0].secretRef.name"))
		})

		it("validates bindings name uniqueness", func() {
			build.Spec.Bindings = []corev1alpha1.CNBBinding{
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

			assertValidationError(build, apis.ErrGeneric("duplicate binding name \"apm\"", "spec.bindings[0].name", "spec.bindings[2].name"))
		})

		it("combining errors", func() {
			build.Spec.Tags = []string{}
			build.Spec.Builder.Image = ""
			assertValidationError(build,
				apis.ErrMissingField("tags").ViaField("spec").
					Also(apis.ErrMissingField("image").ViaField("spec", "builder")))
		})

		it("validates spec is immutable", func() {
			original := build.DeepCopy()

			build.Spec.Source.Git.URL = "http://something/different"
			err := build.Validate(apis.WithinUpdate(context.TODO(), original))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "http://something/different")

		})
	})
}
