package dockercreds

import (
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestDockerCreds(t *testing.T) {
	spec.Run(t, "DockerCreds", testDockerCreds)
}

func testDockerCreds(t *testing.T, when spec.G, it spec.S) {
	var testPullSecretsDir string

	it.Before(func() {
		var err error
		testPullSecretsDir, err = ioutil.TempDir("", "test.append")
		require.NoError(t, err)
	})

	it.After(func() {
		os.RemoveAll(testPullSecretsDir)
	})

	it("adds secrets to config.json", func() {
		err := ioutil.WriteFile(filepath.Join(testPullSecretsDir, "config.json"), []byte(`{
        "auths": {
                "https://index.docker.io/v1/": {
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
                }
        }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		credsToAppend := DockerCreds{
			"gcr.io": entry{
				Auth: "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
			},
		}

		expectedConfigJsonContents := `{
        "auths": {
                "https://index.docker.io/v1/": {
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
                },
				"gcr.io": {
						"auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo="
				}
        }
}`
		err = credsToAppend.AppendCredsToDockerConfig(filepath.Join(testPullSecretsDir, "config.json"))
		require.NoError(t, err)

		configJsonBytes, err := ioutil.ReadFile(filepath.Join(testPullSecretsDir, "config.json"))
		require.NoError(t, err)

		assert.JSONEq(t, expectedConfigJsonContents, string(configJsonBytes))
	})

	it("writes a new config.json does not exist", func() {
		credsToAppend := DockerCreds{
			"gcr.io": entry{
				Auth: "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
			},
		}

		expectedConfigJsonContents := `{
        "auths": {
				"gcr.io": {
						"auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo="
				}
        }
}`
		err := credsToAppend.AppendCredsToDockerConfig(filepath.Join(testPullSecretsDir, "config.json"))
		require.NoError(t, err)

		configJsonBytes, err := ioutil.ReadFile(filepath.Join(testPullSecretsDir, "config.json"))
		require.NoError(t, err)

		assert.JSONEq(t, expectedConfigJsonContents, string(configJsonBytes))

	})

	it("does not overwrite registries if they already exist", func() {
		err := ioutil.WriteFile(filepath.Join(testPullSecretsDir, "config.json"), []byte(`{
        "auths": {
                "https://index.docker.io/v1/": {
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
                }
        }
}`,
		), os.ModePerm)
		require.NoError(t, err)

		credsToAppend := DockerCreds{
			"https://index.docker.io/v1/": entry{
				Auth: "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
			},
		}

		expectedConfigJsonContents := `{
        "auths": {
                "https://index.docker.io/v1/": {
                        "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHNpbGxpbmVzcwo="
                }
        }
}`
		err = credsToAppend.AppendCredsToDockerConfig(filepath.Join(testPullSecretsDir, "config.json"))
		require.NoError(t, err)

		configJsonBytes, err := ioutil.ReadFile(filepath.Join(testPullSecretsDir, "config.json"))
		require.NoError(t, err)

		assert.JSONEq(t, expectedConfigJsonContents, string(configJsonBytes))
	})
}
