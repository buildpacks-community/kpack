package dockercreds

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
)

func TestParsePullSecrets(t *testing.T) {
	spec.Run(t, "Parse Pull Secrets", parsePullSecrets)
}

func parsePullSecrets(t *testing.T, when spec.G, it spec.S) {
	var testPullSecretsDir string

	it.Before(func() {
		var err error
		testPullSecretsDir, err = ioutil.TempDir("", "test.pullsecrets")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testPullSecretsDir))
	})

	it("parses .dockerconfigjson", func() {
		err := ioutil.WriteFile(filepath.Join(testPullSecretsDir, ".dockerconfigjson"), []byte(`{
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

		creds, err := ParseDockerPullSecrets(testPullSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": entry{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)

	})

	it("parses .dockercfg", func() {
		err := ioutil.WriteFile(filepath.Join(testPullSecretsDir, ".dockercfg"), []byte(`{
  "https://index.docker.io/v1/": {
    "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
    "username": "testusername",
    "password": "testpassword"
  }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerPullSecrets(testPullSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": entry{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})

	it("parses .dockercfg and .dockerconfigjson", func() {
		err := ioutil.WriteFile(filepath.Join(testPullSecretsDir, ".dockerconfigjson"), []byte(`{
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

		err = ioutil.WriteFile(filepath.Join(testPullSecretsDir, ".dockercfg"), []byte(`{
  "gcr.io": {
    "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
    "username": "testusername",
    "password": "testpassword"
  }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerPullSecrets(testPullSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": entry{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
				Username: "testdockerhub",
				Password: "testdockerhubusername",
			},
			"gcr.io": entry{
				Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
				Username: "testusername",
				Password: "testpassword",
			},
		}
		require.Equal(t, expectedCreds, creds)
	})
}
