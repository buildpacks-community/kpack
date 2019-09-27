package git_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/git/gitfakes"
)

func TestGitKeychain(t *testing.T) {
	spec.Run(t, "Test Git Keychain", testGitKeychain)
}

func testGitKeychain(t *testing.T, when spec.G, it spec.S) {
	fakeSecretReader := &gitfakes.FakeSecretReader{}
	keychain := git.NewGitKeychain(
		[]string{
			"github-creds=https://github.com",
			"noscheme-creds=noschemegit.com"},
		fakeSecretReader)

	when("Resolve", func() {
		it("returns git Auth for matching secrets", func() {
			fakeSecretReader.FromSecretReturns(&git.BasicAuth{
				Username: "saved-username",
				Password: "saved-password",
			}, nil)
			auth, err := keychain.Resolve("https://github.com/org/repo")
			require.NoError(t, err)

			require.Equal(t, auth, &git.BasicAuth{
				Username: "saved-username",
				Password: "saved-password",
			})
		})

		it("returns git Auth for matching secrets without scheme", func() {
			fakeSecretReader.FromSecretReturns(&git.BasicAuth{
				Username: "noschemegit-username",
				Password: "noschemegit-password",
			}, nil)
			auth, err := keychain.Resolve("https://noschemegit.com/org/repo")
			require.NoError(t, err)

			require.Equal(t, auth, &git.BasicAuth{
				Username: "noschemegit-username",
				Password: "noschemegit-password",
			})
		})

		it("returns anonymous Auth for no matching secret", func() {
			auth, err := keychain.Resolve("https://no-creds-github.com/org/repo")
			require.NoError(t, err)

			require.Equal(t, auth, git.AnonymousAuth{})
		})
	})
}
