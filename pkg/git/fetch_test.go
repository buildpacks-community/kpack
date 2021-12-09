package git

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"

	"github.com/BurntSushi/toml"
	git2go "github.com/libgit2/git2go/v33"
	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
)

func TestGitCheckout(t *testing.T) {
	spec.Run(t, "Test Describe Image", testGitCheckout)
}

func testGitCheckout(t *testing.T, when spec.G, it spec.S) {
	when("#Fetch", func() {
		outputBuffer := &bytes.Buffer{}
		fetcher := Fetcher{
			Logger:   log.New(outputBuffer, "", 0),
			Keychain: fakeGitKeychain{},
		}
		var testDir string
		var metadataDir string

		it.Before(func() {
			var err error
			testDir, err = ioutil.TempDir("", "test-git")
			require.NoError(t, err)

			metadataDir, err = ioutil.TempDir("", "test-git")
			require.NoError(t, err)
		})

		it.After(func() {
			require.NoError(t, os.RemoveAll(testDir))
			require.NoError(t, os.RemoveAll(metadataDir))
		})

		testFetch := func(gitUrl, revision string) func() {
			return func() {
				err := fetcher.Fetch(testDir, gitUrl, revision, metadataDir)
				require.NoError(t, err)

				repository, err := git2go.InitRepository(testDir, false)
				require.NoError(t, err)
				defer repository.Free()

				empty, err := repository.IsEmpty()
				require.NoError(t, err)
				require.False(t, empty)

				state := repository.State()
				require.Equal(t, state, git2go.RepositoryStateNone)

				require.Contains(t, outputBuffer.String(), fmt.Sprintf("Successfully cloned \"%s\" @ \"%s\"", gitUrl, revision))

				require.FileExists(t, path.Join(metadataDir, "project-metadata.toml"))

				var projectMetadata project
				_, err = toml.DecodeFile(path.Join(metadataDir, "project-metadata.toml"), &projectMetadata)
				require.NoError(t, err)

				require.Equal(t, "git", projectMetadata.Source.Type)
				require.Equal(t, gitUrl, projectMetadata.Source.Metadata.Repository)
				require.Equal(t, revision, projectMetadata.Source.Metadata.Revision)

				head, err := repository.Head()
				require.NoError(t, err)
				defer head.Free()
				require.Equal(t, head.Target().String(), projectMetadata.Source.Version.Commit)
			}
		}

		it("fetches remote HEAD", testFetch("https://github.com/git-fixtures/basic", "master"))

		it("fetches a branch", testFetch("https://github.com/git-fixtures/basic", "branch"))

		it("fetches a tag", testFetch("https://github.com/git-fixtures/tags", "lightweight-tag"))

		it("fetches a revision", testFetch("https://github.com/git-fixtures/basic", "b029517f6300c2da0f4b651b8642506cd6aaf45d"))

		it("returns error on non-existent ref", func() {
			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/basic", "doesnotexist", metadataDir)
			require.EqualError(t, err, "could not find reference: doesnotexist")
		})

		it("returns error from remote fetch when authentication required", func() {
			err := fetcher.Fetch(testDir, "git@bitbucket.com:org/repo", "main", metadataDir)
			require.EqualError(t, err, "fetching remote: no auth available")
		})

		it("uses the http proxy env vars", func() {
			require.NoError(t, os.Setenv("HTTPS_PROXY", "http://invalid-proxy"))
			defer os.Unsetenv("HTTPS_PROXY")
			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/basic", "master", metadataDir)
			require.Error(t, err)
			require.Contains(t, err.Error(), "fetching remote: failed to resolve address for invalid-proxy")
		})
	})
}

type fakeGitKeychain struct{}

func (f fakeGitKeychain) Resolve(url string, usernameFromUrl string, allowedTypes git2go.CredentialType) (Git2GoCredential, error) {
	return nil, errors.New("no auth available")
}
