package dockercreds

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
				matcher := RegistryMatcher{Registry: regFormat}
				assert.True(t, matcher.Match("reg.io"))
			})

			it("does not match other registries with "+regFormat, func() {
				matcher := RegistryMatcher{Registry: regFormat}
				assert.False(t, matcher.Match("gcr.io"))
			})
		}

		it("only matches on fully qualified dockerhub references", func() {
			matcher := RegistryMatcher{Registry: "https://index.docker.io/v1/"}
			assert.True(t, matcher.Match("index.docker.io"))

			matcher = RegistryMatcher{Registry: "index.docker.io"}
			assert.False(t, matcher.Match("index.docker.io"))
		})
	})
}
