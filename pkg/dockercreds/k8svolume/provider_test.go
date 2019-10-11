package k8svolume_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/kubernetes/pkg/credentialprovider"

	"github.com/pivotal/kpack/pkg/dockercreds/k8svolume"
)

func TestProvider(t *testing.T) {
	spec.Run(t, "Test K8s Volume", testProvider)
}

func testProvider(t *testing.T, when spec.G, it spec.S) {
	var (
		subject = &k8svolume.VolumeSecretProvider{}
	)

	it.After(func() {
		err := os.Unsetenv(k8svolume.SecretFilePathEnv)
		require.NoError(t, err)
	})

	when("#Enabled", func() {
		it("returns true when CREDENTIAL_PROVIDER_SECRET_PATH is present", func() {
			err := os.Setenv(k8svolume.SecretFilePathEnv, "/some/generic/path")
			require.NoError(t, err)

			assert.True(t, subject.Enabled())
		})

		it("returns false when CREDENTIAL_PROVIDER_SECRET_PATH is not present", func() {
			assert.False(t, subject.Enabled())
		})
	})

	when("#Provide", func() {
		var (
			folderDir string
		)
		it.Before(func() {
			var err error
			folderDir, err = ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)
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
			require.NoError(t, os.Setenv(k8svolume.SecretFilePathEnv, folderDir))
			dockerConfig := subject.Provide()
			require.NotNil(t, dockerConfig)
			assert.Equal(t, credentialprovider.DockerConfig{
				"registry.io": {
					Username: "some-username",
					Password: "some-password",
				},
			}, dockerConfig)
		})

		it("returns no configuration when file cannot be found", func() {
			require.NoError(t, os.Setenv(k8svolume.SecretFilePathEnv, filepath.Join(folderDir, "creds")))
			dockerConfig := subject.Provide()
			require.Nil(t, dockerConfig)
		})
	})
}
