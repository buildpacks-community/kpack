package v1alpha1_test

import (
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestImageBuilds(t *testing.T) {
	spec.Run(t, "Image Build Needed", testImageBuilds)
}

func testImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &v1alpha1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: v1alpha1.ImageSpec{
			Image:          "some/image",
			ServiceAccount: "some/service-account",
			BuilderRef:     "some/builder",
			Build: v1alpha1.ImageBuild{
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

	sourceResolver := &v1alpha1.SourceResolver{
		Status: v1alpha1.SourceResolverStatus{
			Status: duckv1alpha1.Status{
				ObservedGeneration: 0,
				Conditions: []duckv1alpha1.Condition{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: v1.ConditionTrue,
					},
				},
			},
			ResolvedSource: v1alpha1.ResolvedSource{
				Git: v1alpha1.ResolvedGitSource{
					URL:      "https://some.git/url",
					Revision: "revision",
					Type:     v1alpha1.Commit,
				},
			},
		},
	}

	build := &v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: v1alpha1.BuildSpec{
			Image:          "some/image",
			Builder:        "some/builder",
			ServiceAccount: "some/serviceaccount",
			Source: v1alpha1.Source{
				Git: v1alpha1.Git{
					URL:      "https://some.git/url",
					Revision: "revision",
				},
			},
		},
		Status: v1alpha1.BuildStatus{
			BuildMetadata: []v1alpha1.BuildpackMetadata{
				{ID: "buildpack.matches", Version: "1"},
			},
		},
	}

	builder := &v1alpha1.Builder{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Status: v1alpha1.BuilderStatus{
			BuilderMetadata: []v1alpha1.BuildpackMetadata{
				{ID: "buildpack.matches", Version: "1"},
			},
		},
	}

	when("#BuildNeeded", func() {
		it("false for no changes", func() {
			assert.False(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		it("true for different image", func() {
			image.Spec.Image = "different"

			assert.True(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		it("true for different GitURL", func() {
			sourceResolver.Status.ResolvedSource.Git.URL = "different"

			assert.True(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		it("true for different GitRevision", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"

			assert.True(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		it("false if source resolver is not ready", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.Status.Conditions = []duckv1alpha1.Condition{
				{
					Type:   duckv1alpha1.ConditionReady,
					Status: v1.ConditionFalse,
				}}

			assert.False(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		it("false if source resolver has not resolved", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.Status.Conditions = []duckv1alpha1.Condition{}

			assert.False(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		it("false if source resolver has not resolved and there is no previous build", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.Status.Conditions = []duckv1alpha1.Condition{}

			assert.False(t, image.BuildNeeded(sourceResolver, nil, builder))
		})

		it("false if source resolver has not processed current generation", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.ObjectMeta.Generation = 2
			sourceResolver.Status.ObservedGeneration = 1

			assert.False(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		it("false for different ServiceAccount", func() {
			image.Spec.ServiceAccount = "different"

			assert.False(t, image.BuildNeeded(sourceResolver, build, builder))
		})

		when("Builder Metadata changes", func() {

			it("false if builder has additional unused buildpack metadata", func() {
				builder.Status.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{ID: "buildpack.matches", Version: "1"},
					{ID: "buildpack.unused", Version: "unused"},
				}

				assert.False(t, image.BuildNeeded(sourceResolver, build, builder))
			})

			it("true if builder metadata has different buildpack from used buildpack", func() {
				builder.Status.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{ID: "buildpack.matches", Version: "NEW_VERSION"},
					{ID: "buildpack.different", Version: "different"},
				}

				assert.True(t, image.BuildNeeded(sourceResolver, build, builder))

			})

			it("true if builder does not have all most recent used buildpacks and is not currently building", func() {
				builder.Status.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{ID: "buildpack.only.new.buildpacks", Version: "1"},
					{ID: "buildpack.only.new.or.unused.buildpacks", Version: "1"},
				}

				assert.True(t, image.BuildNeeded(sourceResolver, build, builder))
			})
		})
	})

	when("#Build", func() {
		it("generates a build name with build number", func() {
			image.Name = "imageName"

			build := image.Build(sourceResolver, builder)

			assert.Contains(t, build.Name, "imageName-build-1-")
			assert.Contains(t, build.Spec.Source.Git.URL, "https://some.git/url")
			assert.Contains(t, build.Spec.Source.Git.Revision, "revision")
		})

		it("with excludes additional images names when explicitly disabled", func() {
			image.Spec.Image = "imagename/foo:test"
			image.Spec.DisableAdditionalImageNames = true
			build := image.Build(sourceResolver, builder)
			require.Len(t, build.Spec.AdditionalImageNames, 0)
		})

		when("generates additional image names for a provided build number", func() {
			it("with tag prefix if image name has a tag", func() {
				image.Spec.Image = "gcr.io/imagename/foo:test"
				build := image.Build(sourceResolver, builder)
				require.Len(t, build.Spec.AdditionalImageNames, 1)
				require.Regexp(t, "gcr.io/imagename/foo:test-b1\\.\\d{8}\\.\\d{6}", build.Spec.AdditionalImageNames[0])
			})

			it("without tag prefix if image name has no provided tag", func() {
				image.Spec.Image = "gcr.io/imagename/notags"
				build := image.Build(sourceResolver, builder)

				require.Len(t, build.Spec.AdditionalImageNames, 1)
				require.Regexp(t, "gcr.io/imagename/notags:b1\\.\\d{8}\\.\\d{6}", build.Spec.AdditionalImageNames[0])
			})

			it("without tag prefix if image name has the tag 'latest' provided", func() {
				image.Spec.Image = "gcr.io/imagename/tagged:latest"
				build := image.Build(sourceResolver, builder)

				require.Len(t, build.Spec.AdditionalImageNames, 1)
				require.Regexp(t, "gcr.io/imagename/tagged:b1\\.\\d{8}\\.\\d{6}", build.Spec.AdditionalImageNames[0])
			})
		})

		it("generates a build name less than 64 characters", func() {
			image.Name = "long-image-name-1234567890-1234567890-1234567890-1234567890-1234567890"

			build := image.Build(sourceResolver, builder)

			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
		})

		it("adds the env vars to the build spec", func() {
			build := image.Build(sourceResolver, builder)

			assert.Equal(t, image.Spec.Build.Env, build.Spec.Env)
		})
	})
}
