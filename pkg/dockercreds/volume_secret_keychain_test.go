package dockercreds_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/dockercreds"
)

func TestVolumeSecretKeychain(t *testing.T) {
	spec.Run(t, "Test K8s Volume", testVolumeSecretKeychain)
}

func testVolumeSecretKeychain(t *testing.T, when spec.G, it spec.S) {

	it.After(func() {
		err := os.Unsetenv(dockercreds.SecretFilePathEnv)
		require.NoError(t, err)
	})

	when("#NewVolumeSecretKeychain", func() {
		var (
			folderDir string
			resource  authn.Resource
		)
		it.Before(func() {
			var err error
			folderDir, err = ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)
			resource = fakeDockerResource{registryString: "registry.io"}
		})

		it.After(func() {
			require.NoError(t, os.RemoveAll(folderDir))
		})

		it("retrieves the configuration from file", func() {
			require.NoError(t, ioutil.WriteFile(filepath.Join(folderDir, ".dockerconfigjson"), []byte(`
{
	"auths": {
		"registry.io": {
			"username": "some-username",
            "password": "some-password"
		}
	}
}`), 0600))

			require.NoError(t, os.Setenv(dockercreds.SecretFilePathEnv, folderDir))
			keychain, err := dockercreds.NewVolumeSecretKeychain()
			require.NoError(t, err)

			resolved, err := keychain.Resolve(resource)
			require.NoError(t, err)
			require.NotNil(t, resolved)
			authn.FromConfig(authn.AuthConfig{
				Username: "some-username",
				Password: "some-password",
			})

			assert.Equal(t, authn.FromConfig(authn.AuthConfig{
				Auth:     "c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk",
				Username: "some-username",
				Password: "some-password",
			}), resolved)
		})

		it("returns anonymous auth cannot be found", func() {
			require.NoError(t, os.Setenv(dockercreds.SecretFilePathEnv, filepath.Join(folderDir, "creds")))
			keychain, err := dockercreds.NewVolumeSecretKeychain()
			require.NoError(t, err)

			auth, err := keychain.Resolve(resource)
			require.NoError(t, err)
			require.Equal(t, auth, authn.Anonymous)
		})
	})
}

type fakeDockerResource struct {
	registryString string
}

func (d fakeDockerResource) String() string {
	return d.registryString
}

func (d fakeDockerResource) RegistryStr() string {
	return d.registryString
}
