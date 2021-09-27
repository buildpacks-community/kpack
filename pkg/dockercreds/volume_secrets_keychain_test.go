package dockercreds_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
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
		require.NoError(t, os.Unsetenv(dockercreds.SecretFilePathsEnv))
		require.NoError(t, os.Unsetenv(dockercreds.SecretFilePathEnv))
	})

	when("#NewVolumeSecretKeychain", func() {
		var (
			tempDir, dir1, dir2  string
			resource1, resource2 authn.Resource
		)

		it.Before(func() {
			var err error
			tempDir, err = ioutil.TempDir("", "tmp")
			require.NoError(t, err)
			dir1 = filepath.Join(tempDir, "dir1")
			os.MkdirAll(dir1, 0777)
			dir2 = filepath.Join(tempDir, "dir2")
			os.MkdirAll(dir2, 0777)
			resource1 = fakeDockerResource{registryString: "registry.io"}
			resource2 = fakeDockerResource{registryString: "other-registry.io"}
		})

		it.After(func() {
			require.NoError(t, os.RemoveAll(tempDir))
		})

		it("retrieves the configuration from any number of files", func() {
			require.NoError(t, ioutil.WriteFile(filepath.Join(dir1, ".dockerconfigjson"), []byte(`
{
	"auths": {
		"registry.io": {
			"username": "some-username",
            "password": "some-password"
		}
	}
}`), 0600))

			require.NoError(t, ioutil.WriteFile(filepath.Join(dir2, ".dockerconfigjson"), []byte(`
{
	"auths": {
		"other-registry.io": {
			"username": "other-username",
            "password": "other-password"
		}
	}
}`), 0600))

			require.NoError(t, os.Setenv(dockercreds.SecretFilePathsEnv, strings.Join([]string{dir1, dir2}, ",")))
			keychain, err := dockercreds.NewVolumeSecretsKeychain()
			require.NoError(t, err)

			resolved1, err := keychain.Resolve(resource1)
			require.NoError(t, err)
			require.NotNil(t, resolved1)
			resolved2, err := keychain.Resolve(resource2)
			require.NoError(t, err)
			require.NotNil(t, resolved2)

			assert.Equal(t, authn.FromConfig(authn.AuthConfig{
				Username: "some-username",
				Password: "some-password",
			}), resolved1)
			assert.Equal(t, authn.FromConfig(authn.AuthConfig{
				Username: "other-username",
				Password: "other-password",
			}), resolved2)
		})

		it("falls back to the historical env var for the file path", func() {
			require.NoError(t, ioutil.WriteFile(filepath.Join(dir1, ".dockerconfigjson"), []byte(`
{
	"auths": {
		"registry.io": {
			"username": "some-username",
            "password": "some-password"
		}
	}
}`), 0600))

			require.NoError(t, os.Setenv(dockercreds.SecretFilePathEnv, dir1))
			keychain, err := dockercreds.NewVolumeSecretsKeychain()
			require.NoError(t, err)

			resolved1, err := keychain.Resolve(resource1)
			require.NoError(t, err)
			require.NotNil(t, resolved1)

			assert.Equal(t, authn.FromConfig(authn.AuthConfig{
				Username: "some-username",
				Password: "some-password",
			}), resolved1)
		})

		it("returns anonymous auth cannot be found", func() {
			require.NoError(t, os.Setenv(dockercreds.SecretFilePathsEnv, filepath.Join(dir1, "creds")))
			keychain, err := dockercreds.NewVolumeSecretsKeychain()
			require.NoError(t, err)

			auth, err := keychain.Resolve(resource1)
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
