package build

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestCompressor(t *testing.T) {
	spec.Run(t, "Build Metadata Compressor", testMetadataDecompressor)
}

func testMetadataDecompressor(t *testing.T, when spec.G, it spec.S) {
	it("uses gzip to compress and decompress build metadata", func() {
		decompressor := &GzipMetadataCompressor{}
		originalMetadata := &BuildStatusMetadata{
			BuildpackMetadata: []corev1alpha1.BuildpackMetadata{{
				Id:       "some-id",
				Version:  "some-version",
				Homepage: "some-homepage",
			}},
			LatestImage:   "some-image",
			StackRunImage: "some-run-image",
			StackID:       "some-id",
		}
		compressedData, err := decompressor.Compress(originalMetadata)
		require.NoError(t, err)

		metadata, err := decompressor.Decompress(compressedData)
		require.NoError(t, err)
		require.Equal(t, originalMetadata, metadata)
	})
}
