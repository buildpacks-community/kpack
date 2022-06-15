package cnb_test

import (
	"fmt"
	"testing"

	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestBuildMetadata(t *testing.T) {
	spec.Run(t, "Metadata Retriever", testMetadataRetriever)
	spec.Run(t, "Metadata Compression", testMetadataCompression)
}

func testMetadataRetriever(t *testing.T, when spec.G, it spec.S) {
	const (
		appTag           = "reg.io/appimage/name"
		cacheTag         = "reg.io/cacheimage/name"
		stackID          = "io.buildpacks.stack.bionic"
		stackDigest      = "sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"
		buildpackVersion = "1.2.3"
		buildpackID      = "test.id"
	)
	var (
		lifecycleImageLabelTemplate = fmt.Sprintf(`{
  "app": %s,
  "runImage": {
    "topLayer": "sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409",
    "reference": "localhost:5000/node@%s"
  },
  "stack": {
    "runImage": {
      "image": "gcr.io:443/run:full-cnb"
    }
  }
}`, "%s", stackDigest)
	)

	when("RemoteMetadataRetriever", func() {
		var (
			fakeKeychain         *registryfakes.FakeKeychain
			appImage, cacheImage ggcrv1.Image
			retriever            *cnb.RemoteMetadataRetriever
			imageFetcher         *registryfakes.FakeClient
		)

		when("GetBuildMetadata", func() {
			it.Before(func() {
				imageFetcher = registryfakes.NewFakeClient()
				fakeKeychain = &registryfakes.FakeKeychain{}

				appImage = randomImage(t)
				appImage, _ = imagehelpers.SetStringLabel(appImage, "io.buildpacks.build.metadata", fmt.Sprintf(`{"buildpacks": [{"id": "%s", "version": "%s"}]}`, buildpackID, buildpackVersion))
				appImage, _ = imagehelpers.SetStringLabel(appImage, "io.buildpacks.stack.id", stackID)

				cacheImage = randomImage(t)
				imageFetcher.AddImage(cacheTag, cacheImage, fakeKeychain)
				retriever = &cnb.RemoteMetadataRetriever{
					ImageFetcher: imageFetcher,
				}
			})

			when("retrieving metadata", func() {
				const lifecycle06AppKeyValue = `[
    {
      "sha": "sha256:919f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409"
    },
    {
      "sha": "sha256:119f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409"
    }
  ]`

				var appDigest string

				it.Before(func() {
					appImage, _ = imagehelpers.SetStringLabel(appImage, "io.buildpacks.lifecycle.metadata", fmt.Sprintf(lifecycleImageLabelTemplate, lifecycle06AppKeyValue))

					d, err := appImage.Digest()
					require.NoError(t, err)
					appDigest = d.String()
					imageFetcher.AddImage(appTag, appImage, fakeKeychain)
				})

				it("retrieves the metadata from the image and cache tag", func() {
					metadata, err := retriever.GetBuildMetadata(appTag, cacheTag, fakeKeychain)
					assert.NoError(t, err)

					bpMetadata := metadata.BuildpackMetadata
					require.Len(t, bpMetadata, 1)
					assert.Equal(t, buildpackID, bpMetadata[0].Id)
					assert.Equal(t, buildpackVersion, bpMetadata[0].Version)

					assert.Equal(t, fmt.Sprintf("gcr.io:443/run@%s", stackDigest), metadata.StackRunImage)
					assert.Equal(t, stackID, metadata.StackID)

					assert.Equal(t, fmt.Sprintf("%s@%s", appTag, appDigest), metadata.LatestImage)
					cacheDigest, err := cacheImage.Digest()
					require.NoError(t, err)
					assert.Equal(t, fmt.Sprintf("%s@%s", cacheTag, cacheDigest.String()), metadata.LatestCacheImage)
				})

				it("does not error for bad cache tag", func() {
					metadata, err := retriever.GetBuildMetadata(appTag, "invalid", fakeKeychain)
					assert.NoError(t, err)
					assert.Empty(t, metadata.LatestCacheImage)
				})
			})

			when("images are built with lifecycle 0.5", func() {
				const lifecycle05AppKeyValue = `{"sha": "sha256:119f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409"}`

				it("retrieves the metadata from the registry", func() {
					appImage, _ = imagehelpers.SetStringLabel(appImage, "io.buildpacks.lifecycle.metadata", fmt.Sprintf(lifecycleImageLabelTemplate, lifecycle05AppKeyValue))
					d, err := appImage.Digest()
					require.NoError(t, err)
					imageRef := fmt.Sprintf("%s@%s", appTag, d.String())
					imageFetcher.AddImage(imageRef, appImage, fakeKeychain)
					metadata, err := retriever.GetBuildMetadata(imageRef, cacheTag, fakeKeychain)
					assert.NoError(t, err)

					bpMetadata := metadata.BuildpackMetadata
					require.Len(t, bpMetadata, 1)
					assert.Equal(t, buildpackID, bpMetadata[0].Id)
					assert.Equal(t, buildpackVersion, bpMetadata[0].Version)

					assert.Equal(t, fmt.Sprintf("gcr.io:443/run@%s", stackDigest), metadata.StackRunImage)
					assert.Equal(t, stackID, metadata.StackID)

					appDigest, err := appImage.Digest()
					require.NoError(t, err)
					assert.Equal(t, fmt.Sprintf("%s@%s", appTag, appDigest.String()), metadata.LatestImage)
					cacheDigest, err := cacheImage.Digest()
					require.NoError(t, err)
					assert.Equal(t, fmt.Sprintf("%s@%s", cacheTag, cacheDigest.String()), metadata.LatestCacheImage)
				})
			})
		})
	})
}

func testMetadataCompression(t *testing.T, when spec.G, it spec.S) {
	it("uses gzip to serialize/deserialize and compress/decompress build metadata", func() {
		originalMetadata := &cnb.BuildMetadata{
			BuildpackMetadata: []corev1alpha1.BuildpackMetadata{{
				Id:       "some-id",
				Version:  "some-version",
				Homepage: "some-homepage",
			}},
			LatestImage:   "some-image",
			StackRunImage: "some-run-image",
			StackID:       "some-id",
		}
		compressedData, err := cnb.CompressBuildMetadata(originalMetadata)
		require.NoError(t, err)

		metadata, err := cnb.DecompressBuildMetadata(string(compressedData))
		require.NoError(t, err)
		require.Equal(t, originalMetadata, metadata)
	})

	it("errors when data is larger than max termination msg size", func() {
		tooBig := string(make([]byte, 1000000))
		originalMetadata := &cnb.BuildMetadata{
			BuildpackMetadata: []corev1alpha1.BuildpackMetadata{{
				Id:       "some-id",
				Version:  "some-version",
				Homepage: "some-homepage",
			}},
			LatestImage:   tooBig,
			StackRunImage: "some-run-image",
			StackID:       "some-id",
		}
		_, err := cnb.CompressBuildMetadata(originalMetadata)
		require.EqualError(t, err, "compressed metadata size too large")
	})
}

func randomImage(t *testing.T) ggcrv1.Image {
	image, err := random.Image(5, 10)
	require.NoError(t, err)
	return image
}
