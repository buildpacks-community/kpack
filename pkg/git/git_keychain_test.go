package git

import (
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/sclevine/spec"
)

func TestGitFileKeychain(t *testing.T) {
	spec.Run(t, "Test Git Keychain", testGitFileKeychain)
}

func testGitFileKeychain(t *testing.T, when spec.G, it spec.S) {

	var testDir string
	var keychain GitKeychain

	it.Before(func() {
		var err error
		testDir, err = ioutil.TempDir("", "git-keychain")
		require.NoError(t, err)

		require.NoError(t, os.MkdirAll(path.Join(testDir, "github-creds"), 0777))
		require.NoError(t, os.MkdirAll(path.Join(testDir, "more-github-creds"), 0777))
		require.NoError(t, os.MkdirAll(path.Join(testDir, "bitbucket-creds"), 0777))
		require.NoError(t, os.MkdirAll(path.Join(testDir, "basic-bitbucket-creds"), 0777))
		require.NoError(t, os.MkdirAll(path.Join(testDir, "zzz-ssh-bitbucket-creds"), 0777))
		require.NoError(t, os.MkdirAll(path.Join(testDir, "noscheme-creds"), 0777))
		require.NoError(t, os.MkdirAll(path.Join(testDir, "git-ssh-creds"), 0777))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "github-creds", corev1.BasicAuthUsernameKey), []byte("saved-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "github-creds", corev1.BasicAuthPasswordKey), []byte("saved-password"), 0600))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "more-github-creds", corev1.BasicAuthUsernameKey), []byte("another-saved-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "more-github-creds", corev1.BasicAuthPasswordKey), []byte("another-saved-password"), 0600))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "bitbucket-creds", corev1.SSHAuthPrivateKey), []byte("private key 1"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "zzz-ssh-bitbucket-creds", corev1.SSHAuthPrivateKey), []byte("private key 2"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "git-ssh-creds", corev1.SSHAuthPrivateKey), []byte("private key 3"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "basic-bitbucket-creds", corev1.BasicAuthUsernameKey), []byte("saved-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "basic-bitbucket-creds", corev1.BasicAuthPasswordKey), []byte("saved-password"), 0600))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "noscheme-creds", corev1.BasicAuthUsernameKey), []byte("noschemegit-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "noscheme-creds", corev1.BasicAuthPasswordKey), []byte("noschemegit-password"), 0600))

		keychain, err = NewMountedSecretGitKeychain(testDir, []string{
			"github-creds=https://github.com",
			"more-github-creds=https://github.com",
			"basic-bitbucket-creds=https://bitbucket.com",
			"noscheme-creds=noschemegit.com"}, []string{
			"zzz-ssh-bitbucket-creds=https://bitbucket.com",
			"bitbucket-creds=https://bitbucket.com",
		})
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testDir))
	})

	when("Resolve", func() {
		it("returns alphabetical first git Auth for matching basic auth secrets", func() {
			cred, err := keychain.Resolve("https://github.com/org/repo", "", CredentialTypeUserpass)
			require.NoError(t, err)

			require.Equal(t, &GoGitHttpCredential{User: "saved-username", Password: "saved-password"}, cred)
			gogitCred, err := cred.Cred()
			require.NoError(t, err)

			require.Equal(t, reflect.TypeOf(gogitCred).Elem().String(), reflect.TypeOf(http.BasicAuth{}).String())
		})

		it("returns git Auth for matching secrets without scheme", func() {
			cred, err := keychain.Resolve("https://noschemegit.com/org/repo", "", CredentialTypeUserpass)
			require.NoError(t, err)

			require.Equal(t, &GoGitHttpCredential{User: "noschemegit-username", Password: "noschemegit-password"}, cred)
		})

		when("there are ssh and basic auth secret types", func() {
			it("returns ssh cred for requested ssh credentials", func() {
				cred, err := keychain.Resolve("git@bitbucket.com:org/repo", "git", CredentialTypeSSHKey)
				require.NoError(t, err)

				require.Equal(t, &GoGitSshCredential{User: "git", PrivateKey: []byte("private key 1")}, cred)
			})

			it("returns basic auth secret for requested basic auth credentials", func() {
				cred, err := keychain.Resolve("https://bitbucket.com/org/repo", "git", CredentialTypeUserpass)
				require.NoError(t, err)

				require.Equal(t, &GoGitHttpCredential{User: "saved-username", Password: "saved-password"}, cred)
			})
		})

		it("returns an error if no credentials found", func() {
			_, err := keychain.Resolve("https://no-creds-github.com/org/repo", "git", CredentialTypeUserpass)
			require.EqualError(t, err, "no credentials found for https://no-creds-github.com/org/repo")
		})

		when("ssh usernameFromUrl is empty during credential callback", func() {
			it("determines correct username", func() {
				gitKeychain, err := NewMountedSecretGitKeychain(testDir, []string{}, []string{
					"git-ssh-creds=git@my-git-server.com",
				})
				cred, err := gitKeychain.Resolve("ssh://git@my-git-server.com/my-org/my-repo.git", "", CredentialTypeSSHKey)
				require.NoError(t, err)

				require.Equal(t, &GoGitSshCredential{User: "git", PrivateKey: []byte("private key 3")}, cred)
			})
		})
	})
}
