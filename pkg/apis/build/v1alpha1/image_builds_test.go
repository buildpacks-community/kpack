package v1alpha1

import (
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestImageBuilds(t *testing.T) {
	spec.Run(t, "Image build Needed", testImageBuilds)
}

func testImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: ImageSpec{
			Image:          "some/image",
			ServiceAccount: "some/service-account",
			BuilderRef:     "some/builder",
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

	sourceResolver := &SourceResolver{
		Status: SourceResolverStatus{
			Status: duckv1alpha1.Status{
				ObservedGeneration: 0,
				Conditions: []duckv1alpha1.Condition{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: v1.ConditionTrue,
					},
				},
			},
			ResolvedSource: ResolvedSource{
				Git: ResolvedGitSource{
					URL:      "https://some.git/url",
					Revision: "revision",
					Type:     Commit,
				},
			},
		},
	}

	build := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: BuildSpec{
			Image:          "some/image",
			Builder:        "some/builder",
			ServiceAccount: "some/serviceaccount",
			Source: Source{
				Git: Git{
					URL:      "https://some.git/url",
					Revision: "revision",
				},
			},
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
		Status: BuildStatus{
			BuildMetadata: []BuildpackMetadata{
				{ID: "buildpack.matches", Version: "1"},
			},
		},
	}

	builder := &Builder{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Status: BuilderStatus{
			BuilderMetadata: []BuildpackMetadata{
				{ID: "buildpack.matches", Version: "1"},
			},
		},
	}

	when("#buildNeeded", func() {
		it("false for no changes", func() {
			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.False(t, needed)
			assert.Len(t, reasons, 0)
		})

		it("true for different image", func() {
			image.Spec.Image = "different"

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.True(t, needed)
			assert.Len(t, reasons, 1)
			assert.Contains(t, reasons, BuildReasonConfig)
		})

		it("true for different GitURL", func() {
			sourceResolver.Status.ResolvedSource.Git.URL = "different"

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.True(t, needed)
			assert.Len(t, reasons, 1)
			assert.Contains(t, reasons, BuildReasonConfig)
		})

		it("true for different GitRevision", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.True(t, needed)
			assert.Len(t, reasons, 1)
			assert.Contains(t, reasons, BuildReasonCommit)
		})

		it("false if source resolver is not ready", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.Status.Conditions = []duckv1alpha1.Condition{
				{
					Type:   duckv1alpha1.ConditionReady,
					Status: v1.ConditionFalse,
				}}

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.False(t, needed)
			assert.Len(t, reasons, 0)
		})

		it("false if source resolver has not resolved", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.Status.Conditions = []duckv1alpha1.Condition{}

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.False(t, needed)
			assert.Len(t, reasons, 0)
		})

		it("false if source resolver has not resolved and there is no previous build", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.Status.Conditions = []duckv1alpha1.Condition{}

			reasons, needed := image.buildNeeded(nil, sourceResolver, builder)
			assert.False(t, needed)
			assert.Len(t, reasons, 0)
		})

		it("false if source resolver has not processed current generation", func() {
			sourceResolver.Status.ResolvedSource.Git.Revision = "different"
			sourceResolver.ObjectMeta.Generation = 2
			sourceResolver.Status.ObservedGeneration = 1

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.False(t, needed)
			assert.Len(t, reasons, 0)
		})

		it("false for different ServiceAccount", func() {
			image.Spec.ServiceAccount = "different"

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.False(t, needed)
			assert.Len(t, reasons, 0)
		})

		it("true if build env changes", func() {
			build.Spec.Env = []v1.EnvVar{
				{Name: "keyA", Value: "previous-value"},
			}

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.True(t, needed)
			assert.Len(t, reasons, 1)
			assert.Contains(t, reasons, BuildReasonConfig)
		})

		it("true if build env order changes and git url changes", func() {
			build.Spec.Source.Git.URL = "old-git.com/url"
			build.Spec.Env = []v1.EnvVar{
				{Name: "keyA", Value: "old"},
			}
			image.Spec.Build.Env = []v1.EnvVar{
				{Name: "keyA", Value: "new"},
			}

			reasons, needed := image.buildNeeded(build, sourceResolver, builder)
			assert.True(t, needed)
			assert.Len(t, reasons, 1)
		})

		when("Builder Metadata changes", func() {

			it("false if builder has additional unused buildpack metadata", func() {
				builder.Status.BuilderMetadata = []BuildpackMetadata{
					{ID: "buildpack.matches", Version: "1"},
					{ID: "buildpack.unused", Version: "unused"},
				}

				reasons, needed := image.buildNeeded(build, sourceResolver, builder)
				assert.False(t, needed)
				assert.Len(t, reasons, 0)
			})

			it("true if builder metadata has different buildpack from used buildpack", func() {
				builder.Status.BuilderMetadata = []BuildpackMetadata{
					{ID: "buildpack.matches", Version: "NEW_VERSION"},
					{ID: "buildpack.different", Version: "different"},
				}

				reasons, needed := image.buildNeeded(build, sourceResolver, builder)
				assert.True(t, needed)
				assert.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonBuildpack)
			})

			it("true if builder does not have all most recent used buildpacks and is not currently building", func() {
				builder.Status.BuilderMetadata = []BuildpackMetadata{
					{ID: "buildpack.only.new.buildpacks", Version: "1"},
					{ID: "buildpack.only.new.or.unused.buildpacks", Version: "1"},
				}

				reasons, needed := image.buildNeeded(build, sourceResolver, builder)
				assert.True(t, needed)
				assert.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonBuildpack)
			})

			it("true if both config and commit have changed", func() {
				sourceResolver.Status.ResolvedSource.Git.URL = "different"
				sourceResolver.Status.ResolvedSource.Git.Revision = "different"

				reasons, needed := image.buildNeeded(build, sourceResolver, builder)
				assert.True(t, needed)
				assert.Len(t, reasons, 2)
				assert.Contains(t, reasons, BuildReasonConfig)
				assert.Contains(t, reasons, BuildReasonCommit)
			})
		})
	})

	when("#build", func() {
		it("generates a build name with build number", func() {
			image.Name = "imageName"

			build := image.build(sourceResolver, builder, []string{}, 27)

			assert.Contains(t, build.GenerateName, "imageName-build-27-")
			assert.Contains(t, build.Spec.Source.Git.URL, "https://some.git/url")
			assert.Contains(t, build.Spec.Source.Git.Revision, "revision")
		})

		it("with excludes additional images names when explicitly disabled", func() {
			image.Spec.Image = "imagename/foo:test"
			image.Spec.DisableAdditionalImageNames = true
			build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)
			require.Len(t, build.Spec.AdditionalImageNames, 0)
		})

		when("generates additional image names for a provided build number", func() {
			it("with tag prefix if image name has a tag", func() {
				image.Spec.Image = "gcr.io/imagename/foo:test"
				build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 45)
				require.Len(t, build.Spec.AdditionalImageNames, 1)
				require.Regexp(t, "gcr.io/imagename/foo:test-b45\\.\\d{8}\\.\\d{6}", build.Spec.AdditionalImageNames[0])
			})

			it("without tag prefix if image name has no provided tag", func() {
				image.Spec.Image = "gcr.io/imagename/notags"
				build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

				require.Len(t, build.Spec.AdditionalImageNames, 1)
				require.Regexp(t, "gcr.io/imagename/notags:b1\\.\\d{8}\\.\\d{6}", build.Spec.AdditionalImageNames[0])
			})

			it("without tag prefix if image name has the tag 'latest' provided", func() {
				image.Spec.Image = "gcr.io/imagename/tagged:latest"
				build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

				require.Len(t, build.Spec.AdditionalImageNames, 1)
				require.Regexp(t, "gcr.io/imagename/tagged:b1\\.\\d{8}\\.\\d{6}", build.Spec.AdditionalImageNames[0])
			})
		})

		it("generates a build name less than 64 characters", func() {
			image.Name = "long-image-name-1234567890-1234567890-1234567890-1234567890-1234567890"

			build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
		})

		it("adds the env vars to the build spec", func() {
			build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

			assert.Equal(t, image.Spec.Build.Env, build.Spec.Env)
		})

		it("adds build reasons annotation", func() {
			build := image.build(sourceResolver, builder, []string{BuildReasonConfig, BuildReasonCommit}, 1)

			assert.Equal(t, "CONFIG,COMMIT", build.Annotations[BuildReasonAnnotation])
		})
	})
}
