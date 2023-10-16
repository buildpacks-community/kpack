package dockercreds

import (
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
		testSecretsDir, err = os.MkdirTemp("", "test.pullsecrets")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testSecretsDir))
	})

	it("parses .dockerconfigjson favoring auth key", func() {
		err := os.WriteFile(filepath.Join(testSecretsDir, ".dockerconfigjson"), []byte(`{
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
				Password: "testpasswordsilliness\n",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})

	it("parses .dockerconfigjson setting auth key to username/password if unset", func() {
		err := os.WriteFile(filepath.Join(testSecretsDir, ".dockerconfigjson"), []byte(`{
  "auths": {
    "https://index.docker.io/v1/": {
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
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZA==",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})

	it("parses .dockercfg favoring auth key", func() {
		err := os.WriteFile(filepath.Join(testSecretsDir, ".dockercfg"), []byte(`{
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
				Password: "testpasswordsilliness\n",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})

	it("parses .dockercfg setting auth key to username/password if unset", func() {
		err := os.WriteFile(filepath.Join(testSecretsDir, ".dockercfg"), []byte(`{
  "https://index.docker.io/v1/": {
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
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZA==",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})

	it("parses .dockercfg and .dockerconfigjson", func() {
		err := os.WriteFile(filepath.Join(testSecretsDir, ".dockerconfigjson"), []byte(`{
  "auths": {
    "https://index.docker.io/v1/": {
      "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
    }
  }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(testSecretsDir, ".dockercfg"), []byte(`{
  "gcr.io": {
    "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZA==",
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
				Password: "testpasswordsilliness\n",
			},
			"gcr.io": authn.AuthConfig{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZA==",
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
			testDir, err = os.MkdirTemp("", "docker-secret-parse-test")
			require.NoError(t, err)

			require.NoError(t, os.MkdirAll(path.Join(testDir, "gcr-creds"), 0777))
			require.NoError(t, os.MkdirAll(path.Join(testDir, "dockerhub-creds"), 0777))

			require.NoError(t, os.WriteFile(path.Join(testDir, "gcr-creds", corev1.BasicAuthUsernameKey), []byte("gcr-username"), 0600))
			require.NoError(t, os.WriteFile(path.Join(testDir, "gcr-creds", corev1.BasicAuthPasswordKey), []byte("gcr-password"), 0600))

			require.NoError(t, os.WriteFile(path.Join(testDir, "dockerhub-creds", corev1.BasicAuthUsernameKey), []byte("dockerhub-username"), 0600))
			require.NoError(t, os.WriteFile(path.Join(testDir, "dockerhub-creds", corev1.BasicAuthPasswordKey), []byte("dockerhub-password"), 0600))

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
