package git_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/git"
)

func TestGitKeychain(t *testing.T) {
	spec.Run(t, "Test Git Keychain", testGitKeychain)
}

func testGitKeychain(t *testing.T, when spec.G, it spec.S) {

	var testDir string
	var keychain git.GitKeychain
	it.Before(func() {
		var err error
		testDir, err = ioutil.TempDir("", "git-keychain")
		require.NoError(t, err)

		require.NoError(t, os.MkdirAll(path.Join(testDir, "github-creds"), 0777))
		require.NoError(t, os.MkdirAll(path.Join(testDir, "noscheme-creds"), 0777))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "github-creds", corev1.BasicAuthUsernameKey), []byte("saved-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "github-creds", corev1.BasicAuthPasswordKey), []byte("saved-password"), 0600))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "noscheme-creds", corev1.BasicAuthUsernameKey), []byte("noschemegit-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "noscheme-creds", corev1.BasicAuthPasswordKey), []byte("noschemegit-password"), 0600))

		keychain, err = git.NewMountedSecretGitKeychain(
			testDir,
			[]string{
				"github-creds=https://github.com",
				"noscheme-creds=noschemegit.com"},
		)
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testDir))
	})

	when("Resolve", func() {
		it("returns git Auth for matching secrets", func() {
			auth, err := keychain.Resolve("https://github.com/org/repo")
			require.NoError(t, err)

			require.Equal(t, auth, &http.BasicAuth{
				Username: "saved-username",
				Password: "saved-password",
			})
		})

		it("returns git Auth for matching secrets without scheme", func() {
			auth, err := keychain.Resolve("https://noschemegit.com/org/repo")
			require.NoError(t, err)

			require.Equal(t, auth, &http.BasicAuth{
				Username: "noschemegit-username",
				Password: "noschemegit-password",
			})
		})

		it("returns anonymous Auth for no matching secret", func() {
			auth, err := keychain.Resolve("https://no-creds-github.com/org/repo")
			require.NoError(t, err)

			require.Nil(t, auth)
		})
	})
}
