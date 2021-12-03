package git

import (
	git2go "github.com/libgit2/git2go/v31"
	giturls "github.com/whilp/git-urls"
	"golang.org/x/net/http/httpproxy"
)

func proxyFromEnv(gitURL string) (git2go.ProxyOptions, error) {
	u, err := giturls.Parse(gitURL)
	if err != nil {
		return git2go.ProxyOptions{}, err
	}

	proxyURL, err := httpproxy.FromEnvironment().ProxyFunc()(u)
	if err != nil {
		return git2go.ProxyOptions{}, err
	}

	if proxyURL == nil {
		return git2go.ProxyOptions{}, nil
	}

	proxyOptions := git2go.ProxyOptions{
		Type: git2go.ProxyTypeSpecified,
		Url:  proxyURL.String(),
	}

	return proxyOptions, nil
}
