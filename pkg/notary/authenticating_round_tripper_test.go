package notary_test

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

	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/notary"
)

func TestAuthenticatingRoundTripper(t *testing.T) {
	spec.Run(t, "Test Authenticating Round Tripper", testAuthenticatingRoundTripper)
}

func testAuthenticatingRoundTripper(t *testing.T, when spec.G, it spec.S) {
	var (
		keychain = dockercreds.DockerCreds{}

		roundTripper = &notary.AuthenticatingRoundTripper{
			Keychain:            keychain,
			WrappedRoundTripper: http.DefaultTransport,
		}
	)

	when("#RoundTrip", func() {
		when("unauthorized", func() {
			it("retrieves an auth token", func() {
				var (
					authReq   *http.Request
					finalReq  *http.Request
					callCount = 0
				)
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch callCount {
					case 1:
						authReq = r
						_, _ = w.Write([]byte(`{ "token": "some-token" }`))
						w.WriteHeader(http.StatusOK)
					case 2:
						finalReq = r
						w.WriteHeader(http.StatusOK)
					default:
						w.Header().Set("www-authenticate", fmt.Sprintf(`Bearer realm="http://%s",service="some-service",scope="some-scope,some-other-scope"`, r.Host))
						w.WriteHeader(http.StatusUnauthorized)
					}
					callCount++
				}))
				defer ts.Close()

				parsedURL, err := url.Parse(ts.URL)
				require.NoError(t, err)

				keychain[parsedURL.Host] = authn.AuthConfig{
					Username: "some-username",
					Password: "some-password",
				}

				req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
				require.NoError(t, err)

				resp, err := roundTripper.RoundTrip(req)
				require.NoError(t, err)

				assert.Equal(t, http.StatusOK, resp.StatusCode)

				assert.Equal(t, "some-service", authReq.URL.Query().Get("service"))
				assert.Equal(t, "some-scope,some-other-scope", authReq.URL.Query().Get("scope"))
				assert.Equal(t, "kpack", authReq.URL.Query().Get("client_id"))
				assert.Equal(t, "Basic c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk", authReq.Header.Get("Authorization"))

				assert.Equal(t, "Bearer some-token", finalReq.Header.Get("Authorization"))
			})
		})

		it("processes the request", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))
			defer ts.Close()

			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			require.NoError(t, err)

			resp, err := roundTripper.RoundTrip(req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	})
}
