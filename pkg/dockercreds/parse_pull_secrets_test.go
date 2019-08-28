package dockercreds

import (
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
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
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
                }
        }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerPullSecrets(testPullSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": entry{
				Auth: "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
			},
		}
		require.Equal(t, expectedCreds, creds)

	})

	it("parses .dockercfg", func() {
		err := ioutil.WriteFile(filepath.Join(testPullSecretsDir, ".dockercfg"), []byte(`{
                "https://index.docker.io/v1/": {
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
                }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerPullSecrets(testPullSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": entry{
				Auth: "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
			},
		}
		require.Equal(t, expectedCreds, creds)

	})
	it("parses .dockercfg and .dockerconfigjson", func() {
		err := ioutil.WriteFile(filepath.Join(testPullSecretsDir, ".dockerconfigjson"), []byte(`{
        "auths": {
                "https://index.docker.io/v1/": {
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
                }
        }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(testPullSecretsDir, ".dockercfg"), []byte(`{
                "gcr.io": {
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo="
                }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		creds, err := ParseDockerPullSecrets(testPullSecretsDir)
		require.NoError(t, err)

		expectedCreds := DockerCreds{
			"https://index.docker.io/v1/": entry{
				Auth: "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo=",
			},
			"gcr.io": entry{
				Auth: "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
			},
		}
		require.Equal(t, expectedCreds, creds)

	})
}
