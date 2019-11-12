package git

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	ssh2 "golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	corev1 "k8s.io/api/core/v1"
)

func TestGitKeychain(t *testing.T) {
	privateKeyBytes := gitTest{key1: generateRandomPrivateKey(t), key2: generateRandomPrivateKey(t)}
	spec.Run(t, "Test Git Keychain", privateKeyBytes.testGitKeychain)
}

func (keys gitTest) testGitKeychain(t *testing.T, when spec.G, it spec.S) {

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

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "github-creds", corev1.BasicAuthUsernameKey), []byte("saved-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "github-creds", corev1.BasicAuthPasswordKey), []byte("saved-password"), 0600))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "more-github-creds", corev1.BasicAuthUsernameKey), []byte("another-saved-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "more-github-creds", corev1.BasicAuthPasswordKey), []byte("another-saved-password"), 0600))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "bitbucket-creds", corev1.SSHAuthPrivateKey), keys.key1, 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "zzz-ssh-bitbucket-creds", corev1.SSHAuthPrivateKey), keys.key2, 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "basic-bitbucket-creds", corev1.BasicAuthUsernameKey), []byte("saved-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "basic-bitbucket-creds", corev1.BasicAuthPasswordKey), []byte("saved-password"), 0600))

		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "noscheme-creds", corev1.BasicAuthUsernameKey), []byte("noschemegit-username"), 0600))
		require.NoError(t, ioutil.WriteFile(path.Join(testDir, "noscheme-creds", corev1.BasicAuthPasswordKey), []byte("noschemegit-password"), 0600))

		keychain, err = NewMountedSecretGitKeychain(testDir, []string{
			"more-github-creds=https://github.com",
			"github-creds=https://github.com",
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
		when("there are multiple secrets for the same repository", func() {
			it("returns the alphabetical first secretRef when they are all the same secret type", func() {

				auth, err := keychain.Resolve("https://github.com/org/repo")
				require.NoError(t, err)

				require.Equal(t, &http.BasicAuth{
					Username: "saved-username",
					Password: "saved-password",
				}, auth)
			})
		})

		when("there are ssh and basic auth secret types", func() {
			it("returns ssh secret if the target is an ssh target", func() {
				auth, err := keychain.Resolve("git@bitbucket.com:org/repo")
				require.NoError(t, err)

				publicKeys, ok := auth.(*ssh.PublicKeys)
				require.True(t, ok)

				require.Equal(t, "git", publicKeys.User)

				expectedSigner, err := ssh2.ParsePrivateKey(keys.key1)
				require.NoError(t, err)
				require.Equal(t, expectedSigner, publicKeys.Signer)

				require.Nil(t, publicKeys.HostKeyCallback("bitbucket.com", nil, expectedSigner.PublicKey()))
			})

			it("returns basic auth secret if the target is an https target", func() {
				auth, err := keychain.Resolve("https://bitbucket.com/org/repo")
				require.NoError(t, err)

				require.NoError(t, err)
				require.Equal(t, &http.BasicAuth{
					Username: "saved-username",
					Password: "saved-password",
				}, auth)
			})
		})

		it("returns git Auth for matching basic auth secrets", func() {
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
