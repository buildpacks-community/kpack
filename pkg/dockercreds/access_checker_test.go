package dockercreds

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessChecker(t *testing.T) {
	spec.Run(t, "Test HasWriteAccess", testAccessChecker)
}

func testAccessChecker(t *testing.T, when spec.G, it spec.S) {
	var (
		handler = http.NewServeMux()
		server  = httptest.NewServer(handler)
		tagName = fmt.Sprintf("%s/some/image:tag", server.URL[7:])
	)

	when("HasWriteAccess", func() {
		it("true when has permission", func() {
			handler.HandleFunc("/v2/some/image/blobs/uploads/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(201)
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			hasAccess, err := HasWriteAccess(testKeychain{}, tagName)
			require.NoError(t, err)
			assert.True(t, hasAccess)
		})

		it("requests scope push permission", func() {
			handler.HandleFunc("/unauthorized-token/", func(writer http.ResponseWriter, request *http.Request) {
				values, err := url.ParseQuery(request.URL.RawQuery)
				require.NoError(t, err)
				assert.Equal(t, "repository:some/image:push,pull", values.Get("scope"))
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("WWW-Authenticate", fmt.Sprintf("bearer realm=%s/unauthorized-token/", server.URL))
				writer.WriteHeader(401)
			})

			_, _ = HasWriteAccess(testKeychain{}, tagName)
		})

		it("false when fetching token is unauthorized", func() {
			handler.HandleFunc("/unauthorized-token/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(401)
				writer.Write([]byte(`{"errors": [{"code":  "UNAUTHORIZED"}]}`))
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("WWW-Authenticate", fmt.Sprintf("bearer realm=%s/unauthorized-token/", server.URL))
				writer.WriteHeader(401)
			})

			tagName := fmt.Sprintf("%s/some/image:tag", server.URL[7:])

			hasAccess, err := HasWriteAccess(testKeychain{}, tagName)
			require.NoError(t, err)
			assert.False(t, hasAccess)
		})

		it("false when server responds with unauthorized but without a code such as on artifactory", func() {
			handler.HandleFunc("/unauthorized-token/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(401)
				writer.Write([]byte(`{"statusCode":401,"details":"BAD_CREDENTIAL"}`))
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("WWW-Authenticate", fmt.Sprintf("bearer realm=%s/unauthorized-token/", server.URL))
				writer.WriteHeader(401)
			})

			tagName := fmt.Sprintf("%s/some/image:tag", server.URL[7:])

			hasAccess, err := HasWriteAccess(testKeychain{}, tagName)
			require.NoError(t, err)
			assert.False(t, hasAccess)
		})

		it("false when does not have permission", func() {
			handler.HandleFunc("/v2/some/image/blobs/uploads/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(403)
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			tagName := fmt.Sprintf("%s/some/image:tag", server.URL[7:])

			hasAccess, err := HasWriteAccess(testKeychain{}, tagName)
			require.NoError(t, err)
			assert.False(t, hasAccess)
		})

		it("false when cannot reach server with an error", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(404)
			})

			tagName := fmt.Sprintf("%s/some/image:tag", server.URL[7:])

			hasAccess, err := HasWriteAccess(testKeychain{}, tagName)
			require.Error(t, err)
			assert.False(t, hasAccess)
		})
	})

	when("#HasReadAccess", func() {
		it("returns true when we do have read access", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			handler.HandleFunc("/v2/some/image/manifests/tag", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			canRead, err := HasReadAccess(testKeychain{}, tagName)
			require.NoError(t, err)
			assert.True(t, canRead)
		})

		it("returns false when we do not have read access", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			handler.HandleFunc("/v2/some/image/manifests/tag", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(401)
			})

			canRead, err := HasReadAccess(testKeychain{}, tagName)
			require.NoError(t, err)
			assert.False(t, canRead)
		})

		it("returns false when server responds with 404", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(404)
			})

			canRead, err := HasReadAccess(testKeychain{}, tagName)
			require.NoError(t, err)
			assert.False(t, canRead)
		})

		it("returns false with error when we cannot reach the server", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(404)
			})

			canRead, err := HasReadAccess(testKeychain{}, "localhost:9999/blj")
			require.Error(t, err)
			assert.False(t, canRead)
		})
	})
}

type testKeychain struct {
}

func (t testKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return &authn.Basic{
		Username: "testUser",
		Password: "testPassword",
	}, nil
}
