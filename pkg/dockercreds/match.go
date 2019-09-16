package dockercreds

import (
	"fmt"
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

func RegistryMatch(parsedRegistry, registry string) bool {
	for _, format := range registryDomains {
		if fmt.Sprintf(format, parsedRegistry) == registry {
			return true
		}
	}
	return false
}
