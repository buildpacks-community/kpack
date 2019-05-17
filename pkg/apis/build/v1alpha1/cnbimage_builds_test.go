package v1alpha1_test

import (
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestCNBImageBuilds(t *testing.T) {
	spec.Run(t, "Image Build Needed", testCNBImageBuilds)
}

func testCNBImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &v1alpha1.CNBImage{
		ObjectMeta: v1.ObjectMeta{
			Name: "cnb-image-name",
		},
		Spec: v1alpha1.CNBImageSpec{
			Image:          "some/image",
			ServiceAccount: "some/service-account",
			Builder:        "some/builder",
			GitURL:         "https://some.git/url",
			GitRevision:    "revision",
		},
	}

	build := &v1alpha1.CNBBuild{
		ObjectMeta: v1.ObjectMeta{
			Name: "cnb-image-name",
		},
		Spec: v1alpha1.CNBBuildSpec{
			Image:          "some/image",
			Builder:        "some/builder",
			ServiceAccount: "some/serviceaccount",
			GitURL:         "https://some.git/url",
			GitRevision:    "revision",
		},
	}

	when("#BuildNeeded", func() {
		it("Needed for different image", func() {
			build.Spec.Image = "different"

			assert.True(t, image.BuildNeeded(build))
		})

		it("Needed for different builder", func() {
			build.Spec.Builder = "different"

			assert.True(t, image.BuildNeeded(build))
		})

		it("Needed for different GitURL", func() {
			build.Spec.GitURL = "different"

			assert.True(t, image.BuildNeeded(build))
		})

		it("Needed for different GitRevision", func() {
			build.Spec.GitRevision = "different"

			assert.True(t, image.BuildNeeded(build))
		})

		it("Not Needed for different ServiceAccount", func() {
			build.Spec.ServiceAccount = "different"

			assert.False(t, image.BuildNeeded(build))
		})
	})
}
