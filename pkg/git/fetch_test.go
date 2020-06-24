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

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
)

func TestGitCheckout(t *testing.T) {
	spec.Run(t, "Test Describe Image", testGitCheckout)
}

func testGitCheckout(t *testing.T, when spec.G, it spec.S) {
	when("#Fetch", func() {
		outpuBuffer := &bytes.Buffer{}
		fetcher := Fetcher{
			Logger:   log.New(outpuBuffer, "", 0),
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

				repository, err := gogit.PlainOpenWithOptions(testDir, &gogit.PlainOpenOptions{})
				require.NoError(t, err)

				worktree, err := repository.Worktree()
				require.NoError(t, err)

				status, err := worktree.Status()
				require.NoError(t, err)

				require.True(t, status.IsClean())

				require.Contains(t, outpuBuffer.String(), fmt.Sprintf("Successfully cloned \"%s\" @ \"%s\"", gitUrl, revision))

				require.FileExists(t, path.Join(metadataDir, "project-metadata.toml"))

				var projectMetadata project
				_, err = toml.DecodeFile(path.Join(metadataDir, "project-metadata.toml"), &projectMetadata)
				require.NoError(t, err)

				require.Equal(t, "git", projectMetadata.Source.Type)
				require.Equal(t, gitUrl, projectMetadata.Source.Metadata.Repository)
				require.Equal(t, revision, projectMetadata.Source.Metadata.Revision)

				h, err := repository.Head()
				require.NoError(t, err)
				require.Equal(t, h.Hash().String(), projectMetadata.Source.Version.Commit)
			}
		}

		it("fetches remote HEAD", testFetch("https://github.com/git-fixtures/basic", "master"))

		it("fetches a branch", testFetch("https://github.com/git-fixtures/basic", "branch"))

		it("fetches a tag", testFetch("https://github.com/git-fixtures/tags", "lightweight-tag"))

		it("fetches a revision", testFetch("https://github.com/git-fixtures/basic", "b029517f6300c2da0f4b651b8642506cd6aaf45d"))

		it("returns invalid credentials to fetch error on authentication required", func() {
			err := fetcher.Fetch(testDir, "http://github.com/pivotal/kpack-nonexistent-test-repo", "master", "")
			require.EqualError(t, err, "invalid credentials to fetch git repository: http://github.com/pivotal/kpack-nonexistent-test-repo")
		})
	})
}

type fakeGitKeychain struct{}

func (f fakeGitKeychain) Resolve(gitUrl string) (transport.AuthMethod, error) {
	return nil, nil
}
