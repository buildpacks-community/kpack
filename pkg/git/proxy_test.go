package git

import (
	"os"
	"testing"

	git2go "github.com/libgit2/git2go/v31"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
)

type envVar struct {
	key, val string
}

type gitProxyTest struct {
	description, gitURL, expectedProxyURL string
	envVars                               []envVar
}

func (g gitProxyTest) run(t *testing.T, it spec.S) {
	it(g.description, func() {
		for _, e := range g.envVars {
			require.NoError(t, os.Setenv(e.key, e.val))
			defer os.Unsetenv(e.key)
		}
		var expectedOptions git2go.ProxyOptions
		if g.expectedProxyURL != "" {
			expectedOptions = git2go.ProxyOptions{
				Type: git2go.ProxyTypeSpecified,
				Url:  g.expectedProxyURL,
			}
		}
		actualOptions, err := proxyFromEnv(g.gitURL)
		require.NoError(t, err)
		require.Equal(t, expectedOptions, actualOptions)
	})
}

func TestProxyFromEnv(t *testing.T) {
	spec.Run(t, "Test proxyFromEnv", testProxyFromEnv)
}

func testProxyFromEnv(t *testing.T, when spec.G, it spec.S) {
	tests := []gitProxyTest{
		{
			description:      "uses HTTPS_PROXY when url is https",
			envVars:          []envVar{{key: "HTTPS_PROXY", val: "some-https-proxy"}, {key: "HTTP_PROXY", val: "some-http-proxy"}},
			gitURL:           "https://github.com/foo/bar",
			expectedProxyURL: "http://some-https-proxy",
		},
		{
			description:      "uses HTTP_PROXY when url is http",
			envVars:          []envVar{{key: "HTTPS_PROXY", val: "some-https-proxy"}, {key: "HTTP_PROXY", val: "some-http-proxy"}},
			gitURL:           "http://github.com/foo/bar",
			expectedProxyURL: "http://some-http-proxy",
		},
		{
			description:      "uses http_proxy when url is http",
			envVars:          []envVar{{key: "https_proxy", val: "some-https-proxy"}, {key: "http_proxy", val: "some-http-proxy"}},
			gitURL:           "https://github.com/foo/bar",
			expectedProxyURL: "http://some-https-proxy",
		},
		{
			description:      "uses http_proxy when url is http",
			envVars:          []envVar{{key: "https_proxy", val: "some-https-proxy"}, {key: "http_proxy", val: "some-http-proxy"}},
			gitURL:           "http://github.com/foo/bar",
			expectedProxyURL: "http://some-http-proxy",
		},
		{
			description: "uses uppercase env var when upper and lowercase env vars are set",
			envVars: []envVar{
				{key: "HTTPS_PROXY", val: "some-big-https-proxy"},
				{key: "https_proxy", val: "some-small-https-proxy"},
			},
			gitURL:           "https://github.com/foo/bar",
			expectedProxyURL: "http://some-big-https-proxy",
		},
		{
			description:      "uses no proxy settings when no env is set",
			envVars:          []envVar{},
			gitURL:           "https://github.com/foo/bar",
			expectedProxyURL: "",
		},
		{
			description:      "uses no proxy settings when url is ssh",
			envVars:          []envVar{{key: "HTTPS_PROXY", val: "some-https-proxy"}, {key: "HTTP_PROXY", val: "some-http-proxy"}},
			gitURL:           "git@bitbucket.com:org/repo",
			expectedProxyURL: "",
		},
	}
	for _, test := range tests {
		test.run(t, it)
	}
}
