package v1alpha1_test

import (
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestImageBuilds(t *testing.T) {
	spec.Run(t, "Image Build Needed", testImageBuilds)
}

func testImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &v1alpha1.Image{
		ObjectMeta: v1.ObjectMeta{
			Name: "image-name",
		},
		Spec: v1alpha1.ImageSpec{
			Image:          "some/image",
			ServiceAccount: "some/service-account",
			BuilderRef:     "some/builder",
			Source: v1alpha1.Source{
				Git: v1alpha1.Git{
					URL:      "https://some.git/url",
					Revision: "revision",
				},
			},
		},
	}

	build := &v1alpha1.Build{
		ObjectMeta: v1.ObjectMeta{
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
		ObjectMeta: v1.ObjectMeta{
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
			assert.False(t, image.BuildNeeded(build, builder))
		})

		it("true for different image", func() {
			build.Spec.Image = "different"

			assert.True(t, image.BuildNeeded(build, builder))
		})

		it("true for different GitURL", func() {
			build.Spec.Source.Git.URL = "different"

			assert.True(t, image.BuildNeeded(build, builder))
		})

		it("true for different GitRevision", func() {
			build.Spec.Source.Git.Revision = "different"

			assert.True(t, image.BuildNeeded(build, builder))
		})

		it("false for different ServiceAccount", func() {
			build.Spec.ServiceAccount = "different"

			assert.False(t, image.BuildNeeded(build, builder))
		})

		when("Builder Metadata changes", func() {

			it("false if builder has additional unused buildpack metadata", func() {
				builder.Status.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{ID: "buildpack.matches", Version: "1"},
					{ID: "buildpack.unused", Version: "unused"},
				}

				assert.False(t, image.BuildNeeded(build, builder))
			})

			it("true if builder metadata has different buildpack from used buildpack", func() {
				builder.Status.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{ID: "buildpack.matches", Version: "NEW_VERSION"},
					{ID: "buildpack.different", Version: "different"},
				}

				assert.True(t, image.BuildNeeded(build, builder))

			})

			it("true if builder does not have all most recent used buildpacks and is not currently building", func() {
				builder.Status.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{ID: "buildpack.only.new.buildpacks", Version: "1"},
					{ID: "buildpack.only.new.or.unused.buildpacks", Version: "1"},
				}

				assert.True(t, image.BuildNeeded(build, builder))
			})
		})
	})

	when("#CreateBuild", func() {
		it("generates a build name with build number", func() {
			image.Name = "imageName"

			build := image.CreateBuild(builder)

			assert.Contains(t, build.Name, "imageName-build-1-")
		})

		it("generates a build name less than 64 characters", func() {
			image.Name = "long-image-name-1234567890-1234567890-1234567890-1234567890-1234567890"

			build := image.CreateBuild(builder)

			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
		})
	})
}
