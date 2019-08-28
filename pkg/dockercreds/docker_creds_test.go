package dockercreds

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, os.RemoveAll(testPullSecretsDir))
	})

	when("#AppendToDockerConfig", func() {

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
			err = credsToAppend.AppendToDockerConfig(filepath.Join(testPullSecretsDir, "config.json"))
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
			err := credsToAppend.AppendToDockerConfig(filepath.Join(testPullSecretsDir, "config.json"))
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
			err = credsToAppend.AppendToDockerConfig(filepath.Join(testPullSecretsDir, "config.json"))
			require.NoError(t, err)

			configJsonBytes, err := ioutil.ReadFile(filepath.Join(testPullSecretsDir, "config.json"))
			require.NoError(t, err)

			assert.JSONEq(t, expectedConfigJsonContents, string(configJsonBytes))
		})
	})

	when("#Resolve", func() {
		it("returns auth for matching registry", func() {
			creds := DockerCreds{
				"non.match": entry{
					Auth: "no-match=",
				},
				"some.reg": entry{
					Auth: "match-Auth=",
				},
			}

			reference, err := name.ParseReference("some.reg/name", name.WeakValidation)
			require.NoError(t, err)

			auth, err := creds.Resolve(reference.Context().Registry)
			require.NoError(t, err)

			assert.Equal(t, Auth("match-Auth="), auth)
		})

		it("returns Anonymous for no matching registry", func() {
			creds := DockerCreds{
				"non.match": entry{
					Auth: "no-match=",
				},
			}

			reference, err := name.ParseReference("some.reg/name", name.WeakValidation)
			require.NoError(t, err)

			auth, err := creds.Resolve(reference.Context().Registry)
			require.NoError(t, err)

			assert.Equal(t, authn.Anonymous, auth)
		})
	})
}
