package imagehelpers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLazyLayer(t *testing.T) {
	spec.Run(t, "Test Lazy Layer", testLazyLayer)
}

func testLazyLayer(t *testing.T, when spec.G, it spec.S) {
	handler := http.NewServeMux()
	server := httptest.NewServer(handler)
	tagName := fmt.Sprintf("%s/some/image:tag", server.URL[7:])

	var registryCalls []string
	var backingRandomLayer v1.Layer
	var layer v1.Layer

	it.Before(func() {
		var err error

		backingRandomLayer, err = random.Layer(1000, types.DockerLayer)
		require.NoError(t, err)

		digest, err := backingRandomLayer.Digest()
		require.NoError(t, err)

		diffID, err := backingRandomLayer.DiffID()
		require.NoError(t, err)

		size, err := backingRandomLayer.Size()
		require.NoError(t, err)

		handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
			registryCalls = append(registryCalls, request.RequestURI)

			writer.WriteHeader(200)
		})

		handler.HandleFunc(fmt.Sprintf("/v2/some/image/blobs/%s", digest.String()), func(writer http.ResponseWriter, request *http.Request) {
			registryCalls = append(registryCalls, request.RequestURI)

			compressed, err := backingRandomLayer.Compressed()
			require.NoError(t, err)

			io.Copy(writer, compressed)
		})

		layer, err = NewLazyMountableLayer(LazyMountableLayerArgs{
			Digest:   digest.String(),
			DiffId:   diffID.String(),
			Image:    tagName,
			Size:     size,
			Keychain: authn.DefaultKeychain,
		})
		require.NoError(t, err)
	})

	it("is a MountableLayer", func() {
		assert.IsType(t, &remote.MountableLayer{}, layer)

		expectedReference, err := name.ParseReference(tagName)
		require.NoError(t, err)

		assert.Equal(t, expectedReference, layer.(*remote.MountableLayer).Reference)
	})

	it("Digest()", func() {
		digest, err := layer.Digest()
		require.NoError(t, err)

		expectedDigest, err := backingRandomLayer.Digest()
		require.NoError(t, err)

		assert.Equal(t, expectedDigest, digest)
		assert.Empty(t, registryCalls)
	})

	it("Size()", func() {
		size, err := layer.Size()
		require.NoError(t, err)

		expectedSize, err := backingRandomLayer.Size()
		require.NoError(t, err)

		assert.Equal(t, expectedSize, size)
		require.Empty(t, registryCalls)
	})

	it("DiffID()", func() {
		diffID, err := layer.DiffID()
		require.NoError(t, err)

		expectedDiffId, err := backingRandomLayer.DiffID()
		require.NoError(t, err)

		assert.Equal(t, expectedDiffId, diffID)
		require.Empty(t, registryCalls)
	})

	it("MediaType()", func() {
		mediaType, err := layer.MediaType()
		require.NoError(t, err)

		assert.Equal(t, types.DockerLayer, mediaType)
		require.Empty(t, registryCalls)
	})

	it("Compressed()", func() {
		contents, err := layer.Compressed()
		require.NoError(t, err)

		expectedContents, err := backingRandomLayer.Compressed()
		require.NoError(t, err)

		assertEqual(t, expectedContents, contents)
		require.Len(t, registryCalls, 2)
	})

	it("Uncompressed()", func() {
		contents, err := layer.Uncompressed()
		require.NoError(t, err)

		expectedContents, err := backingRandomLayer.Uncompressed()
		require.NoError(t, err)

		assertEqual(t, expectedContents, contents)
		require.Len(t, registryCalls, 2)
	})
}

func assertEqual(t *testing.T, expectedContents io.ReadCloser, contents io.ReadCloser) {
	buffer := &bytes.Buffer{}
	io.Copy(buffer, contents)

	expectedBuffer := &bytes.Buffer{}
	io.Copy(expectedBuffer, expectedContents)

	buffer.String()
	assert.Equal(t, buffer.String(), expectedBuffer.String())
}
