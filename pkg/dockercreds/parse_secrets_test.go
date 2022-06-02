package dockercreds

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestParseDockerConfigSecret(t *testing.T) {
	spec.Run(t, "Parse Docker Config Secret", parseDockerConfigSecret)
}
func TestParseBasicAuthSecrets(t *testing.T) {
	spec.Run(t, "Test Basic Auth Secrets", testParseBasicAuthSecrets)
}

func parseDockerConfigSecret(t *testing.T, when spec.G, it spec.S) {
	var testSecretsDir string

	it.Before(func() {
		var err error
		testSecretsDir, err = ioutil.TempDir("", "test.pullsecrets")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testSecretsDir))
	})

	it("parses .dockerconfigjson", func() {
		err := ioutil.WriteFile(filepath.Join(testSecretsDir, ".dockerconfigjson"), []byte(`{
  "auths": {
    "https://index.docker.io/v1/": {
      "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
      "username": "testusername",
      "password": "testpassword"
    }
  }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerConfigSecret(testSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": authn.AuthConfig{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)

	})

	it("parses .dockercfg", func() {
		err := ioutil.WriteFile(filepath.Join(testSecretsDir, ".dockercfg"), []byte(`{
  "https://index.docker.io/v1/": {
    "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
    "username": "testusername",
    "password": "testpassword"
  }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerConfigSecret(testSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": authn.AuthConfig{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})

	it("parses .dockercfg and .dockerconfigjson", func() {
		err := ioutil.WriteFile(filepath.Join(testSecretsDir, ".dockerconfigjson"), []byte(`{
  "auths": {
    "https://index.docker.io/v1/": {
      "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
      "username": "testdockerhub",
      "password": "testdockerhubusername"
    }
  }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(testSecretsDir, ".dockercfg"), []byte(`{
  "gcr.io": {
    "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
    "username": "testusername",
    "password": "testpassword"
  }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerConfigSecret(testSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": authn.AuthConfig{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
				Username: "testdockerhub",
				Password: "testdockerhubusername",
			},
			"gcr.io": authn.AuthConfig{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})
}

func testParseBasicAuthSecrets(t *testing.T, when spec.G, it spec.S) {
	when("ParseBasicAuthSecrets", func() {

		var testDir string
		it.Before(func() {
			var err error
			testDir, err = ioutil.TempDir("", "docker-secret-parse-test")
			require.NoError(t, err)

			require.NoError(t, os.MkdirAll(path.Join(testDir, "gcr-creds"), 0777))
			require.NoError(t, os.MkdirAll(path.Join(testDir, "dockerhub-creds"), 0777))

			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "gcr-creds", corev1.BasicAuthUsernameKey), []byte("gcr-username"), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "gcr-creds", corev1.BasicAuthPasswordKey), []byte("gcr-password"), 0600))

			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "dockerhub-creds", corev1.BasicAuthUsernameKey), []byte("dockerhub-username"), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "dockerhub-creds", corev1.BasicAuthPasswordKey), []byte("dockerhub-password"), 0600))

		})

		it.After(func() {
			require.NoError(t, os.RemoveAll(testDir))
		})

		when("ParseBasicAuthSecrets", func() {
			it("parses the volume mounted creds", func() {

				creds, err := ParseBasicAuthSecrets(
					testDir,
					[]string{
						"gcr-creds=gcr.io",
						"dockerhub-creds=index.docker.io",
					},
				)
				require.NoError(t, err)

				assert.Equal(t, DockerCreds{
					"gcr.io": {
						Username: "gcr-username",
						Password: "gcr-password",
					},
					"index.docker.io": {
						Username: "dockerhub-username",
						Password: "dockerhub-password",
					},
				}, creds)
			})

		})

	})
}
