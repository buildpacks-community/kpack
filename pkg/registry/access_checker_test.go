package registry_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/registry"
)

func TestHasWriteAccess(t *testing.T) {
	spec.Run(t, "Test HasWriteAccess", testHasWriteAccess)
}

func testHasWriteAccess(t *testing.T, when spec.G, it spec.S) {
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

			hasAccess, err := registry.HasWriteAccess(tagName)
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

			_, _ = registry.HasWriteAccess(tagName)
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

			hasAccess, err := registry.HasWriteAccess(tagName)
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

			hasAccess, err := registry.HasWriteAccess(tagName)
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

			hasAccess, err := registry.HasWriteAccess(tagName)
			require.NoError(t, err)
			assert.False(t, hasAccess)
		})

		it("false when cannot reach server with an error", func() {
			handler.HandleFunc("/v2/", func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(404)
			})

			tagName := fmt.Sprintf("%s/some/image:tag", server.URL[7:])

			hasAccess, err := registry.HasWriteAccess(tagName)
			require.Error(t, err)
			assert.False(t, hasAccess)
		})
	})
}
