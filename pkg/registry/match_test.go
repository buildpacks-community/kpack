package registry

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	spec.Run(t, "RegistryMatch", testRegistryMatch)
}

func testRegistryMatch(t *testing.T, when spec.G, it spec.S) {
	when("#Match", func() {
		for _, regFormat := range []string{
			// Allow naked domains
			"reg.io",
			// Allow scheme-prefixed.
			"https://reg.io",
			"http://reg.io",
			// Allow scheme-prefixes with version in url path.
			"https://reg.io/v1/",
			"http://reg.io/v1/",
			"https://reg.io/v2/",
			"http://reg.io/v2/",
		} {
			it("matches format "+regFormat, func() {
				assert.True(t, registryMatcher{}.Match("reg.io", regFormat))
			})

			it("does not match other registries with "+regFormat, func() {
				assert.False(t, registryMatcher{}.Match("gcr.io", regFormat))
			})
		}

		it("matches on dockerhub references", func() {
			assert.True(t, registryMatcher{}.Match("index.docker.io", "http://index.docker.io"))
			assert.True(t, registryMatcher{}.Match("index.docker.io", "index.docker.io"))
		})
	})
}
