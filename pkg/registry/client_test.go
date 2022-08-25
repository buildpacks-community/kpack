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
	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/reconciler"
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
		keychain = authn.NewMultiKeychain()
		handler  = http.NewServeMux()
		server   = httptest.NewServer(handler)
		tagName  = fmt.Sprintf("%s/some/image:tag", server.URL[7:])
		subject  = &registry.Client{}
	)

	when("Fetch", func() {

		when("no error", func() {
			const digest = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
			const sampleManifest = `{
  "schemaVersion": 2,
  "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
  "config": {
    "mediaType": "application/vnd.docker.container.image.v1+json",
    "size": 48618,
    "digest": "sha256:634bb7adff6f8e5347ccaf8b456682aec43d4d622669406b1b3cefc270d8c317"
  },
  "layers": [
    {
      "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
      "size": 29597783,
      "digest": "sha256:b734f5039f3da6bc4fe77c1295add3b1bbd7ddc9fa328f5fa467ce61acc49535"
    }
  ]
}
`

			it.Before(func() {
				handler.HandleFunc("/v2/some/image/manifests", func(writer http.ResponseWriter, request *http.Request) {
					if request.Method != "GET" {
						t.Errorf("unexpected %s to %s", request.Method, request.URL)
						writer.WriteHeader(500)
						return
					}
					writer.WriteHeader(200)

					writer.Write([]byte(sampleManifest))
				})

				handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
					if request.Method != "GET" {
						t.Errorf("unexpected %s to %s", request.Method, request.URL)
						writer.WriteHeader(404)
						return
					}

					writer.WriteHeader(200)
				})

			})

			when("#Identifer", func() {
				var fullyQualifedImageRef = fmt.Sprintf("%s/some/image@%s", server.URL[7:], digest)
				it("includes digest if repoName does not have a digest", func() {
					_, imageId, err := subject.Fetch(keychain, tagName)
					require.NoError(t, err)

					require.Equal(t, imageId, fullyQualifedImageRef)
				})

				it("includes digest if repoName already has a digest", func() {
					_, imageId, err := subject.Fetch(keychain, fullyQualifedImageRef)
					require.NoError(t, err)

					require.Equal(t, imageId, fullyQualifedImageRef)
				})
			})
		})

		when("error", func() {
			when("network", func() {
				it.Before(func() {
					handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
						writer.WriteHeader(404)
					})
				})
				it("wraps it to NetworkError", func() {
					assertNetworkErrorOn(t, true, func() error {
						_, _, err := subject.Fetch(keychain, tagName)
						return err
					})
				})
			})

			when("unauthorized", func() {
				it.Before(func() {
					handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
						writer.WriteHeader(http.StatusUnauthorized)
					})
				})
				it("doesn't wrap it to NetworkError", func() {
					assertNetworkErrorOn(t, false, func() error {
						_, _, err := subject.Fetch(keychain, tagName)
						return err
					})
				})
			})

			when("forbidden", func() {
				it.Before(func() {
					handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
						writer.WriteHeader(http.StatusForbidden)
					})
				})
				it("doesn't wrap it to NetworkError", func() {
					assertNetworkErrorOn(t, false, func() error {
						_, _, err := subject.Fetch(keychain, tagName)
						return err
					})
				})
			})
		})
	})

	when("Save", func() {
		when("no error", func() {
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

				_, err := subject.Save(keychain, tagName, image)
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

				_, err := subject.Save(keychain, tagName, image)
				require.NoError(t, err)
			})
		})

		when("error", func() {
			when("network", func() {
				it.Before(func() {
					handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
						writer.WriteHeader(http.StatusNotFound)
					})
				})
				it("wraps it to NetworkError", func() {
					assertNetworkErrorOn(t, true, func() error {
						image := randomImage(t, 0)
						_, err := subject.Save(keychain, tagName, image)
						return err
					})
				})
			})

			when("unauthorized", func() {
				it.Before(func() {
					handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
						writer.WriteHeader(http.StatusUnauthorized)
					})
				})
				it("doesn't wrap it to NetworkError", func() {
					assertNetworkErrorOn(t, false, func() error {
						image := randomImage(t, 0)
						_, err := subject.Save(keychain, tagName, image)
						return err
					})
				})
			})

			when("forbidden", func() {
				it.Before(func() {
					handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
						writer.WriteHeader(http.StatusForbidden)
					})
				})
				it("doesn't wrap it to NetworkError", func() {
					assertNetworkErrorOn(t, false, func() error {
						image := randomImage(t, 0)
						_, err := subject.Save(keychain, tagName, image)
						return err
					})
				})
			})
		})
	})
}

func randomImage(t *testing.T, layers int64) v1.Image {
	image, err := random.Image(5, layers)
	require.NoError(t, err)
	return image
}

func assertNetworkErrorOn(t *testing.T, expected bool, fn func() error) {
	err := fn()
	require.Error(t, err)
	var networkError *reconciler.NetworkError
	if expected {
		require.True(t, errors.As(err, &networkError))
	} else {
		require.False(t, errors.As(err, &networkError))
	}
}
