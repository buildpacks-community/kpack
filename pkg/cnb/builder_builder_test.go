package cnb

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
)

func TestBuilderBlder(t *testing.T) {
	spec.Run(t, "TestBuilderBlder", testBuilderBlder)
}

func testBuilderBlder(t *testing.T, when spec.G, it spec.S) {
	var (
		builderBuilder *builderBlder
		kpackVersion   = "0.16.0"
	)

	when("Adding a stack", func() {
		it.Before(func() {
			builderBuilder = newBuilderBldr(kpackVersion)
		})

		it("errors when the stack is windows", func() {
			baseImage := baseImage(t, "windows")
			err := builderBuilder.AddStack(baseImage, &buildapi.ClusterStack{})
			assert.EqualError(t, err, "windows base images are not supported")
		})

		it("copies the resolved clusterstack to the builder", func() {
			resolvedStack := buildapi.ResolvedClusterStack{
				Id:      "some-id",
				Mixins:  []string{"some-mixin"},
				UserID:  1000,
				GroupID: 1000,
			}

			baseImage := baseImage(t, "linux")
			err := builderBuilder.AddStack(baseImage, &buildapi.ClusterStack{Status: buildapi.ClusterStackStatus{ResolvedClusterStack: resolvedStack}})
			assert.NoError(t, err)

			assert.Equal(t, "some-id", builderBuilder.stackId)
			assert.Equal(t, 1000, builderBuilder.cnbUserId)
			assert.Equal(t, 1000, builderBuilder.cnbGroupId)
			assert.Equal(t, []string{"some-mixin"}, builderBuilder.mixins)

		})
	})
}

func baseImage(t *testing.T, os string) v1.Image {
	img, err := random.Image(1, 1)
	assert.NoError(t, err)

	config, err := img.ConfigFile()
	assert.NoError(t, err)

	config.OS = os

	img, err = mutate.ConfigFile(img, config)
	assert.NoError(t, err)

	return img
}
