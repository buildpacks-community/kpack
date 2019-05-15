package registry_test

import (
	"github.com/pivotal/build-service-system/pkg/registry"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
)

func TestMatch(t *testing.T) {
	spec.Run(t, "RegistryMatch", testRegistryMatch)
}

func testRegistryMatch(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect func(interface{}, ...interface{}) GomegaAssertion
	)

	it.Before(func() {
		Expect = NewGomegaWithT(t).Expect
	})

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
				reference, err := name.ParseReference("reg.io/some/name", name.WeakValidation)
				Expect(err).NotTo(HaveOccurred())

				Expect(registry.Match(reference.Context().Registry, regFormat)).To(BeTrue())
			})

			it("does not match other registries with "+regFormat, func() {
				reference, err := name.ParseReference("gcr.io/some/name", name.WeakValidation)
				Expect(err).NotTo(HaveOccurred())

				Expect(registry.Match(reference.Context().Registry, regFormat)).To(BeFalse())
			})
		}

		it("matches on dockerhub references", func() {
			reference, err := name.ParseReference("some/name", name.WeakValidation)
			Expect(err).NotTo(HaveOccurred())

			Expect(registry.Match(reference.Context().Registry, "http://index.docker.io")).To(BeTrue())
			Expect(registry.Match(reference.Context().Registry, "index.docker.io")).To(BeTrue())
		})
	})
}
