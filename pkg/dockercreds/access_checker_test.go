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

	when("VerifyWriteAccess", func() {
		it("does not error when has write access", func() {
			handler.HandleFunc("/v2/some/image/blobs/uploads/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(201)
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			err := VerifyWriteAccess(testKeychain{}, tagName)
			require.NoError(t, err)
		})

		it("requests scope push permission", func() {
			handler.HandleFunc("/unauthorized-token/", func(writer http.ResponseWriter, request *http.Request) {
				values, err := url.ParseQuery(request.URL.RawQuery)
				require.NoError(t, err)
				assert.Equal(t, "repository:some/image:push,pull", values.Get("scope"))
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("WWW-Authenticate", fmt.Sprintf("bearer realm=\"%s/unauthorized-token/\"", server.URL))
				writer.WriteHeader(401)
			})

			_ = VerifyWriteAccess(testKeychain{}, tagName)
		})

		it.Focus("errors when fetching token is unauthorized", func() {
			handler.HandleFunc("/unauthorized-token/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(401)
				writer.Write([]byte(`{"errors": [{"code":  "UNAUTHORIZED"}]}`))
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("WWW-Authenticate", fmt.Sprintf("bearer realm=\"%s/unauthorized-token/\"", server.URL))
				writer.WriteHeader(401)
			})

			err := VerifyWriteAccess(testKeychain{}, tagName)
			assert.EqualError(t, err, "UNAUTHORIZED")
		})

		it("errors when server responds with unauthorized but without a code such as on artifactory", func() {
			handler.HandleFunc("/unauthorized-token/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(401)
				writer.Write([]byte(`{"statusCode":401,"details":"BAD_CREDENTIAL"}`))
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Add("WWW-Authenticate", fmt.Sprintf("bearer realm=\"%s/unauthorized-token/\"", server.URL))
				writer.WriteHeader(401)
			})

			err := VerifyWriteAccess(testKeychain{}, tagName)
			assert.EqualError(t, err, "UNAUTHORIZED")
		})

		it("errors when does not have permission", func() {
			handler.HandleFunc("/v2/some/image/blobs/uploads/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(403)
			})

			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			err := VerifyWriteAccess(testKeychain{}, tagName)
			assert.EqualError(t, err, fmt.Sprintf("POST %s/v2/some/image/blobs/uploads/: unsupported status code 403", server.URL))
		})

		it("errors when cannot reach server", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(404)
			})

			err := VerifyWriteAccess(testKeychain{}, tagName)
			assert.EqualError(t, err, fmt.Sprintf("GET %s/v2/: unsupported status code 404", server.URL))
		})

		it("errors when server errors", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(500)
			})

			err := VerifyWriteAccess(testKeychain{}, tagName)
			assert.EqualError(t, err, fmt.Sprintf("GET %s/v2/: unsupported status code 500", server.URL))
		})
	})

	when("#VerifyReadAccess", func() {
		it("does not error when has read access", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			handler.HandleFunc("/v2/some/image/manifests/tag", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			err := VerifyReadAccess(testKeychain{}, tagName)
			require.NoError(t, err)
		})

		it("errors when has no read access", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(200)
			})

			handler.HandleFunc("/v2/some/image/manifests/tag", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(401)
			})

			err := VerifyReadAccess(testKeychain{}, tagName)
			assert.EqualError(t, err, "UNAUTHORIZED")
		})

		it("errors when cannot reach server", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(404)
			})

			err := VerifyReadAccess(testKeychain{}, tagName)
			assert.EqualError(t, err, fmt.Sprintf("GET %s/v2/: unsupported status code 404 Not Found", server.URL))
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
