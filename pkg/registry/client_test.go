package registry_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/registry"
)

func TestClient(t *testing.T) {
	spec.Run(t, "TestClient", testClient)
}

func testClient(t *testing.T, when spec.G, it spec.S) {
	const (
		layerCount = 0
	)

	var (
		handler = http.NewServeMux()
		server  = httptest.NewServer(handler)
		tagName = fmt.Sprintf("%s/some/image:tag", server.URL[7:])
		subject = &registry.Client{}
	)

	when("Fetch", func() {
		when("#Identifer", func() {
			it("includes digest if repoName does not have a digest", func() {
				_, imageId, err := subject.Fetch(authn.DefaultKeychain, "gcr.io/paketo-buildpacks/builder:base")
				require.NoError(t, err)

				require.Len(t, imageId, 104)
				require.Equal(t, imageId[0:40], "gcr.io/paketo-buildpacks/builder@sha256:")
			})

			it("includes digest if repoName already has a digest", func() {
				_, imageId, err := subject.Fetch(authn.DefaultKeychain, "gcr.io/paketo-buildpacks/builder@sha256:fc6c76f22d6d9d2afd654625b63607453cf3ccb65af485905ddfccd812e9eb97")
				require.NoError(t, err)

				require.Equal(t, imageId, "gcr.io/paketo-buildpacks/builder@sha256:fc6c76f22d6d9d2afd654625b63607453cf3ccb65af485905ddfccd812e9eb97")
			})
		})
	})

	when("Save", func() {

		it("should save", func() {
			image := randomImage(t, layerCount)
			var (
				numberOfLayerUploads       = 0
				numberOfManifestsSaves     = 0
				numberOfAdditionalTagSaves = 0
			)

			handler.HandleFunc("/v2/some/image/blobs/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
				numberOfLayerUploads++
			})

			handler.HandleFunc("/v2/some/image/manifests/tag", func(writer http.ResponseWriter, request *http.Request) {
				if request.Method == "GET" {
					writer.WriteHeader(404)
					return
				}

				numberOfManifestsSaves++
				writer.WriteHeader(201)
			})

			handler.HandleFunc("/v2/some/image/manifests/", func(writer http.ResponseWriter, request *http.Request) {
				if request.Method == "GET" {
					t.Errorf("unexpected %s to %s", request.Method, request.URL)
					writer.WriteHeader(404)
					return
				}
				assert.Regexp(t, regexp.MustCompile("/v2/some/image/manifests/\\d{14}"), request.RequestURI)
				numberOfAdditionalTagSaves++

				writer.WriteHeader(200)
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != "GET" {
					t.Errorf("unexpected %s to %s", request.Method, request.URL)
					writer.WriteHeader(404)
					return
				}

				writer.WriteHeader(200)
			})

			_, err := subject.Save(authn.DefaultKeychain, tagName, image)
			require.NoError(t, err)

			const configLayer = 1
			assert.Equal(t, numberOfLayerUploads, layerCount+configLayer)
			assert.Equal(t, numberOfManifestsSaves, 1)
			assert.Equal(t, numberOfAdditionalTagSaves, 1)
		})

		it("does not save images if exisiting image already exisits", func() {
			image := randomImage(t, layerCount)

			handler.HandleFunc("/v2/some/image/manifests/tag", func(writer http.ResponseWriter, request *http.Request) {
				configFile, err := image.RawManifest()
				require.NoError(t, err)

				writer.Write(configFile)
				writer.WriteHeader(200)
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != "GET" {
					t.Fatalf("unexpected %s to %s", request.Method, request.URL)
				}

				writer.WriteHeader(200)
			})

			_, err := subject.Save(authn.DefaultKeychain, tagName, image)
			require.NoError(t, err)
		})
	})
}

func randomImage(t *testing.T, layers int64) v1.Image {
	image, err := random.Image(5, layers)
	require.NoError(t, err)
	return image
}
