package git

import (
	"os"
	"path"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	ssh2 "golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
)

func TestGitKeychain(t *testing.T) {
	privateKeyBytes := gitTest{key1: generateRandomPrivateKey(t), key2: generateRandomPrivateKey(t)}
	spec.Run(t, "Test Git Keychain", privateKeyBytes.testGitKeychain)
}

func writeSecrets(testDir string, secrets map[string]map[string][]byte) error {
	for name, creds := range secrets {
		err := os.MkdirAll(path.Join(testDir, name), 0777)
		if err != nil {
			return err
		}
		for k, v := range creds {
			err = os.WriteFile(path.Join(testDir, name, k), v, 0600)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (keys gitTest) testGitKeychain(t *testing.T, when spec.G, it spec.S) {

	var testDir string
	var keychain GitKeychain

	it.Before(func() {
		var err error
		testDir, err = os.MkdirTemp("", "git-keychain")
		require.NoError(t, err)

		secrets := map[string]map[string][]byte{
			"github-creds": {
				corev1.BasicAuthUsernameKey: []byte("another-saved-username"),
				corev1.BasicAuthPasswordKey: []byte("another-saved-password"),
			},
			"additional-github-creds": {
				corev1.BasicAuthUsernameKey: []byte("saved-username"),
				corev1.BasicAuthPasswordKey: []byte("saved-password"),
			},
			"bitbucket-creds": {
				corev1.SSHAuthPrivateKey: keys.key1,
			},
			"basic-bitbucket-creds": {
				corev1.BasicAuthUsernameKey: []byte("saved-username"),
				corev1.BasicAuthPasswordKey: []byte("saved-password"),
			},
			"zzz-ssh-bitbucket-creds": {
				corev1.SSHAuthPrivateKey: []byte("private key 2"),
			},
			"noscheme-creds": {
				corev1.BasicAuthUsernameKey: []byte("noschemegit-username"),
				corev1.BasicAuthPasswordKey: []byte("noschemegit-password"),
			},
			"git-ssh-creds": {
				corev1.SSHAuthPrivateKey: []byte("private key 3"),
			},
		}

		require.NoError(t, writeSecrets(testDir, secrets))

		keychain, err = NewMountedSecretGitKeychain(testDir, []string{
			"github-creds=https://github.com",
			"additional-github-creds=https://github.com",
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
			it("returns alphabetical first git Auth for matching basic auth secrets", func() {
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
			})

			it("returns ssh cred for requested ssh credentials", func() {
				auth, err := keychain.Resolve("git@bitbucket.com:org/repo")
				require.NoError(t, err)

				_, ok := auth.(*ssh.PublicKeys)
				require.True(t, ok)

				signer, err := ssh2.ParsePrivateKey(keys.key1)
				require.NoError(t, err)

				require.Equal(t, &ssh.PublicKeys{
					User:   "git",
					Signer: signer,
				}, auth)
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
