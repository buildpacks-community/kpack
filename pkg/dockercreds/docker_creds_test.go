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

	when("#Save", func() {
		it("saves secrets to the provided path in json", func() {
			creds := DockerCreds{
				"gcr.io": entry{
					Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
					Username: "testusername",
					Password: "testpassword",
				},
			}

			expectedConfigJsonContents := `{
  "auths": {
    "gcr.io": {
      "auth": "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
      "username": "testusername",
      "password": "testpassword"
    }
  }
}`
			err := creds.Save(filepath.Join(testPullSecretsDir, "config.json"))
			require.NoError(t, err)

			configJsonBytes, err := ioutil.ReadFile(filepath.Join(testPullSecretsDir, "config.json"))
			require.NoError(t, err)

			assert.JSONEq(t, expectedConfigJsonContents, string(configJsonBytes))
		})
	})

	when("#Append", func() {
		it("creates a new Dockercreds with both creds appended", func() {
			creds := DockerCreds{
				"gcr.io": entry{
					Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
					Username: "testusername",
					Password: "testpassword",
				},
			}

			newCreds, err := creds.Append(DockerCreds{
				"appendedcreds.io": entry{
					Auth:     "AppendedCreds=",
					Username: "appendedUser",
					Password: "appendedPassword",
				},
			})
			require.NoError(t, err)

			assert.Equal(t, newCreds, DockerCreds{
				"gcr.io": entry{
					Auth:     "dGVzdHVzZXJuYW1lOnRlc3RwYXNzd29yZHRoYXR3aWxsbm90d29yawo=",
					Username: "testusername",
					Password: "testpassword",
				},
				"appendedcreds.io": entry{
					Auth:     "AppendedCreds=",
					Username: "appendedUser",
					Password: "appendedPassword",
				},
			})
		})

		it("does not overwrite registries in the appended creds if they already exist", func() {
			creds := DockerCreds{
				"gcr.io": entry{
					Auth: "dontOverwriteMe=",
				},
			}

			newCreds, err := creds.Append(DockerCreds{
				"gcr.io": entry{
					Auth: "ToNotBeOverwritten=",
				},
			})
			require.NoError(t, err)

			assert.Equal(t, newCreds, DockerCreds{
				"gcr.io": entry{
					Auth: "dontOverwriteMe=",
				},
			})
		})

		it("does not overwrite registries if they already exist in a different format", func() {
			creds := DockerCreds{
				"gcr.io": entry{
					Auth: "dontOverwriteMe=",
				},
			}

			newCreds, err := creds.Append(DockerCreds{
				"https://gcr.io": entry{
					Auth: "ToNotOverwrite=",
				},
			})
			require.NoError(t, err)

			assert.Equal(t, newCreds, DockerCreds{
				"gcr.io": entry{
					Auth: "dontOverwriteMe=",
				},
			})
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

		it("returns auth for matching registry with only username and password", func() {
			creds := DockerCreds{
				"non.match": entry{
					Auth: "no-match=",
				},
				"some.reg": entry{
					Username: "testusername",
					Password: "testpassword",
				},
			}

			reference, err := name.ParseReference("some.reg/name", name.WeakValidation)
			require.NoError(t, err)

			auth, err := creds.Resolve(reference.Context().Registry)
			require.NoError(t, err)

			assert.Equal(t, &authn.Basic{Username: "testusername", Password: "testpassword"}, auth)
		})

		it("returns auth for matching registry with no credentials", func() {
			creds := DockerCreds{
				"non.match": entry{
					Auth: "no-match=",
				},
				"some.reg": entry{
					Auth:     "",
					Username: "",
					Password: "",
				},
			}

			reference, err := name.ParseReference("some.reg/name", name.WeakValidation)
			require.NoError(t, err)

			_, err = creds.Resolve(reference.Context().Registry)
			assert.Error(t, err)
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
