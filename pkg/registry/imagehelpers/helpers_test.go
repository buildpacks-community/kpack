package imagehelpers_test

import (
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

func TestImageHelpers(t *testing.T) {
	spec.Run(t, "Image Helpers", testImageHelpers)
}

func testImageHelpers(t *testing.T, when spec.G, it spec.S) {
	var image v1.Image

	it.Before(func() {
		image = randomImage(t)
	})

	when("#GetCreatedAt", func() {
		it("returns created at from the image", func() {
			createdAt, err := imagehelpers.GetCreatedAt(image)
			require.NoError(t, err)
			require.Equal(t, time.Time{}, createdAt)
		})
	})

	when("Labels", func() {
		it("get and set string labels", func() {
			var (
				err      error
				metadata string
				labels   = map[string]string{
					"foo": "bar",
					"goo": "ber",
				}
			)

			image, err = imagehelpers.SetStringLabels(image, labels)
			require.NoError(t, err)

			hasLabel, err := imagehelpers.HasLabel(image, "foo")
			require.NoError(t, err)
			require.True(t, hasLabel)

			hasLabel, err = imagehelpers.HasLabel(image, "not-exists")
			require.NoError(t, err)
			require.False(t, hasLabel)

			metadata, err = imagehelpers.GetStringLabel(image, "foo")
			require.NoError(t, err)
			require.Equal(t, "bar", metadata)

			metadata, err = imagehelpers.GetStringLabel(image, "goo")
			require.NoError(t, err)
			require.Equal(t, "ber", metadata)
		})

		it("get and set labels", func() {
			var (
				err      error
				metadata string
				labels   = map[string]interface{}{
					"foo": "bar",
					"goo": "ber",
				}
			)

			image, err = imagehelpers.SetLabels(image, labels)
			require.NoError(t, err)

			hasLabel, err := imagehelpers.HasLabel(image, "foo")
			require.NoError(t, err)
			require.True(t, hasLabel)

			hasLabel, err = imagehelpers.HasLabel(image, "not-exists")
			require.NoError(t, err)
			require.False(t, hasLabel)

			err = imagehelpers.GetLabel(image, "foo", &metadata)
			require.NoError(t, err)
			require.Equal(t, "bar", metadata)

			err = imagehelpers.GetLabel(image, "goo", &metadata)
			require.NoError(t, err)
			require.Equal(t, "ber", metadata)
		})
	})

	when("Env", func() {
		it("get and set env", func() {
			var (
				err error
				env string
			)

			image, err = imagehelpers.SetEnv(image, "FOO", "BAR")
			require.NoError(t, err)

			env, err = imagehelpers.GetEnv(image, "FOO")
			require.NoError(t, err)
			require.Equal(t, "BAR", env)
		})
	})
}

func randomImage(t *testing.T) v1.Image {
	image, err := random.Image(5, 10)
	require.NoError(t, err)
	return image
}
