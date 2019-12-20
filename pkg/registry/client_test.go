package registry_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/registry"
)

func TestClient(t *testing.T) {
	spec.Run(t, "TestClient", testClient)
}

func testClient(t *testing.T, when spec.G, it spec.S) {
	handler := http.NewServeMux()
	server := httptest.NewServer(handler)
	tagName := fmt.Sprintf("%s/some/image:tag", server.URL[7:])
	subject := &registry.Client{}

	when("Fetch", func() {
		when("#Identifer", func() {
			it("includes digest if repoName does not have a digest", func() {
				_, imageId, err := subject.Fetch(authn.DefaultKeychain, "cloudfoundry/cnb:bionic")
				require.NoError(t, err)

				require.Len(t, imageId, 104)
				require.Equal(t, imageId[0:40], "index.docker.io/cloudfoundry/cnb@sha256:")
			})

			it("includes digest if repoName already has a digest", func() {
				_, imageId, err := subject.Fetch(authn.DefaultKeychain, "cloudfoundry/cnb:bionic@sha256:33c3ad8676530f864d51d78483b510334ccc4f03368f7f5bb9d517ff4cbd630f")
				require.NoError(t, err)

				require.Equal(t, imageId, "index.docker.io/cloudfoundry/cnb@sha256:33c3ad8676530f864d51d78483b510334ccc4f03368f7f5bb9d517ff4cbd630f")
			})
		})
	})

	when("Save", func() {
		it("should save", func() {
			image := randomImage(t)

			handler.HandleFunc("/v2/some/image/blobs/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			handler.HandleFunc("/v2/some/image/manifests/tag", func(writer http.ResponseWriter, request *http.Request) {
				if request.Method == "GET" {
					writer.WriteHeader(404)
				}
				writer.WriteHeader(201)
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

		it("does not save images if exisiting image already exisits", func() {
			image := randomImage(t)

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

func randomImage(t *testing.T) v1.Image {
	image, err := random.Image(5, 10)
	require.NoError(t, err)
	return image
}
