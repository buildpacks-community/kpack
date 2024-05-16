package blob_test

import (
	"fmt"
	"testing"

	"github.com/pivotal/kpack/pkg/blob"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
)

func TestKeychain(t *testing.T) {
	spec.Run(t, "testKeychain", testKeychain)
}

func testKeychain(t *testing.T, when spec.G, it spec.S) {
	var (
		goodKeychain1 = &fakeKeychain{"some-auth", nil, nil}
		goodKeychain2 = &fakeKeychain{"some-other-auth", map[string]string{"some-header": "some-value"}, nil}
		badKeychain1  = &fakeKeychain{"", nil, fmt.Errorf("some-error")}
		badKeychain2  = &fakeKeychain{"", nil, fmt.Errorf("some-other-error")}
	)
	when("multi keychain", func() {
		it("resolves them in order", func() {
			keychain := blob.NewMultiKeychain(
				goodKeychain1,
				goodKeychain2,
			)

			auth, header, err := keychain.Resolve("https://some-url.com")
			require.NoError(t, err)

			require.Equal(t, "some-auth", auth)
			require.Nil(t, header)
		})

		it("returns the first one non-empty result", func() {
			keychain := blob.NewMultiKeychain(
				badKeychain1,
				badKeychain2,
				goodKeychain2,
			)

			auth, header, err := keychain.Resolve("https://some-url.com")
			require.NoError(t, err)

			require.Equal(t, "some-other-auth", auth)
			require.Contains(t, header, "some-header")
		})

		it("errors if no keychain matches", func() {
			keychain := blob.NewMultiKeychain(
				badKeychain1,
				badKeychain2,
			)

			_, _, err := keychain.Resolve("https://some-url.com")
			require.EqualError(t, err, "no keychain matched for 'https://some-url.com'")
		})
	})
}

type fakeKeychain struct {
	auth   string
	header map[string]string
	err    error
}

func (f fakeKeychain) Resolve(_ string) (string, map[string]string, error) {
	return f.auth, f.header, f.err
}
