package dockercreds

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

var registryDomains = []string{
	// Allow naked domains
	"%s",
	// Allow scheme-prefixed.
	"https://%s",
	"http://%s",
	// Allow scheme-prefixes with version in url path.
	"https://%s/v1/",
	"http://%s/v1/",
	"https://%s/v2/",
	"http://%s/v2/",
}

type RegistryMatcher struct {
	Registry string
}

func (m RegistryMatcher) Match(reg string) bool {
	for _, format := range registryDomains {
		if fmt.Sprintf(format, registryString(reg)) == m.Registry {
			return true
		}
	}
	return false
}

func registryString(reg string) string {
	if reg == name.DefaultRegistry {
		return authn.DefaultAuthKey
	}
	return reg
}
