package secret_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/secret"
)

func TestVolumeSecretReader(t *testing.T) {
	spec.Run(t, "Test Volume Secret Reader", testVolumeSecretReader)
}

func testVolumeSecretReader(t *testing.T, when spec.G, it spec.S) {
	when("#readBasicAuthSecret", func() {
		it("returns the username and password from the secret", func() {
			testDir, err := ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)

			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			require.NoError(t, os.MkdirAll(path.Join(testDir, "creds"), 0777))

			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", corev1.BasicAuthUsernameKey), []byte("saved-username"), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", corev1.BasicAuthPasswordKey), []byte("saved-password"), 0600))

			auth, err := secret.ReadBasicAuthSecret(testDir, "creds")
			require.NoError(t, err)

			assert.Equal(t, auth, secret.BasicAuth{
				Username: "saved-username",
				Password: "saved-password",
			})
		})
	})

	when("#readSshSecret", func() {
		it("returns the private key from the secret", func() {
			testDir, err := ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)

			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			require.NoError(t, os.MkdirAll(path.Join(testDir, "creds"), 0777))

			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", corev1.SSHAuthPrivateKey), []byte("foobar"), 0600))

			auth, err := secret.ReadSshSecret(testDir, "creds")
			require.NoError(t, err)

			assert.Equal(t, auth, secret.SSH{
				PrivateKey: "foobar",
			})
		})
	})
}
