package dockercreds

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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
		testPullSecretsDir, err = os.MkdirTemp("", "test.append")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testPullSecretsDir))
	})

	when("#Save", func() {
		it("saves secrets to the provided path in json", func() {
			creds := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZA==",
					Username: "testusername",
					Password: "testpassword",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			expectedConfigJsonContents := `{
  "auths": {
    "gcr.io": {
      "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZA==",
      "username": "testusername",
      "password": "testpassword"
    }
  }
}`
			err := creds.Save(filepath.Join(testPullSecretsDir, "config.json"))
			require.NoError(t, err)

			configJsonBytes, err := os.ReadFile(filepath.Join(testPullSecretsDir, "config.json"))
			require.NoError(t, err)

			assert.JSONEq(t, expectedConfigJsonContents, string(configJsonBytes))
		})
	})

	when("#Append", func() {
		it("creates a new Dockercreds with both creds appended", func() {
			cred1 := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
					Username: "testusername",
					Password: "testpassword",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			cred2 := DockerCreds{
				map[string]authn.AuthConfig{"appendedcreds.io": {
					Auth:     "AppendedCreds=",
					Username: "appendedUser",
					Password: "appendedPassword",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			newCreds, err := cred1.Append(cred2)
			require.NoError(t, err)

			assertedNewCreds := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
					Username: "testusername",
					Password: "testpassword",
				},
					"appendedcreds.io": {
						Auth:     "AppendedCreds=",
						Username: "appendedUser",
						Password: "appendedPassword",
					}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			assert.Equal(t, newCreds, assertedNewCreds)
		})

		it("does not overwrite registries in the appended creds if they already exist", func() {
			cred1 := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth: "dontOverwriteMe=",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			cred2 := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth: "ToNotBeOverwritten=",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			newCreds, err := cred1.Append(cred2)

			require.NoError(t, err)

			assertCred := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth: "dontOverwriteMe=",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			assert.Equal(t, newCreds, assertCred)
		})

		it("does not overwrite registries if they already exist in a different format", func() {
			cred1 := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth: "dontOverwriteMe=",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			cred2 := DockerCreds{
				map[string]authn.AuthConfig{"https://gcr.io": {
					Auth: "ToNotOverwrite=",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			newCreds, err := cred1.Append(cred2)
			require.NoError(t, err)

			assertCred := DockerCreds{
				map[string]authn.AuthConfig{"gcr.io": {
					Auth: "dontOverwriteMe=",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			assert.Equal(t, newCreds, assertCred)
		})
	})

	when("#Resolve", func() {
		it("returns auth for matching registry", func() {
			credMap := map[string]authn.AuthConfig{"non.match": {
				Auth: "no-match=",
			},
				"some.reg": {
					Auth: "match-Auth=",
				}}

			creds := DockerCreds{
				credMap,
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			reference, err := name.ParseReference("some.reg/name", name.WeakValidation)
			require.NoError(t, err)

			auth, err := creds.Resolve(reference.Context().Registry)
			require.NoError(t, err)

			assert.Equal(t, authn.FromConfig(authn.AuthConfig{
				Auth: "match-Auth=",
			}), auth)
		})

		it("returns auth for matching registry with only username and password", func() {
			credMap := map[string]authn.AuthConfig{"non.match": {
				Auth: "no-match=",
			},
				"some.reg": {
					Username: "testusername",
					Password: "testpassword",
				}}

			creds := DockerCreds{
				credMap,
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			reference, err := name.ParseReference("some.reg/name", name.WeakValidation)
			require.NoError(t, err)

			auth, err := creds.Resolve(reference.Context().Registry)
			require.NoError(t, err)

			assert.Equal(t, authn.FromConfig(authn.AuthConfig{
				Username: "testusername",
				Password: "testpassword",
			}), auth)
		})

		it("returns Anonymous for no matching registry", func() {
			creds := DockerCreds{
				map[string]authn.AuthConfig{"non.match": {
					Auth: "no-match=",
				}},
				time.Date(2000, 10, 10, 10, 10, 10, 10, time.UTC),
				"",
			}

			reference, err := name.ParseReference("some.reg/name", name.WeakValidation)
			require.NoError(t, err)

			auth, err := creds.Resolve(reference.Context().Registry)
			require.NoError(t, err)

			assert.Equal(t, authn.Anonymous, auth)
		})
	})
}
