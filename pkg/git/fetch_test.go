package git

import (
	"bytes"
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
		outputBuffer := &bytes.Buffer{}
		fetcher := Fetcher{
			Logger:   log.New(outputBuffer, "", 0),
			Keychain: fakeGitKeychain{},
		}
		var testDir string
		var metadataDir string

		it.Before(func() {
			var err error
			testDir, err = os.MkdirTemp("", "test-git")
			require.NoError(t, err)

			metadataDir, err = os.MkdirTemp("", "test-git")
			require.NoError(t, err)

			require.NoError(t, os.Unsetenv("HTTPS_PROXY"))
		})

		it.After(func() {
			require.NoError(t, os.RemoveAll(testDir))
			require.NoError(t, os.RemoveAll(metadataDir))
		})

		testFetch := func(gitUrl, revision string) func() {
			return func() {
				err := fetcher.Fetch(testDir, gitUrl, revision, metadataDir)
				require.NoError(t, err)

				repository, err := gogit.PlainOpen(testDir)
				require.NoError(t, err)
				require.Contains(t, outputBuffer.String(), "Successfully cloned")

				p := path.Join(metadataDir, "project-metadata.toml")
				require.FileExists(t, p)

				var projectMetadata project
				_, err = toml.DecodeFile(p, &projectMetadata)
				require.NoError(t, err)
				require.Equal(t, "git", projectMetadata.Source.Type)
				require.Equal(t, gitUrl, projectMetadata.Source.Metadata.Repository)
				require.Equal(t, revision, projectMetadata.Source.Metadata.Revision)

				hash, err := repository.ResolveRevision("HEAD")
				require.NoError(t, err)
				require.Equal(t, hash.String(), projectMetadata.Source.Version.Commit)
			}
		}

		it("fetches remote HEAD", testFetch("https://github.com/git-fixtures/basic", "master"))

		it("fetches a branch", testFetch("https://github.com/git-fixtures/basic", "branch"))

		it("fetches a tag", testFetch("https://github.com/git-fixtures/tags", "lightweight-tag"))

		it("fetches a revision", testFetch("https://github.com/git-fixtures/basic", "b029517f6300c2da0f4b651b8642506cd6aaf45d"))

		it("returns error on non-existent ref", func() {
			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/basic", "doesnotexist", metadataDir)
			require.EqualError(t, err, "resolving revision: reference not found")
		})

		it("preserves symbolic links", func() {
			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/symlinks", "master", metadataDir)
			require.NoError(t, err)

			fileInfo, err := os.Lstat(path.Join(testDir, "bar"))
			require.NoError(t, err)
			require.Equal(t, fileInfo.Mode().Type(), os.ModeSymlink)
		})

		it("preserves executable permission", func() {
			err := fetcher.Fetch(testDir, "https://github.com/pivotal/kpack", "main", metadataDir)
			require.NoError(t, err)

			fileInfo, err := os.Lstat(path.Join(testDir, "hack", "apply.sh"))
			require.NoError(t, err)
			require.True(t, isExecutableByAll(fileInfo.Mode()))

			fileInfo, err = os.Lstat(path.Join(testDir, "hack", "tools.go"))
			require.NoError(t, err)
			require.False(t, isExecutableByAny(fileInfo.Mode()))
		})

		it("returns invalid credentials to fetch error on authentication required", func() {
			err := fetcher.Fetch(testDir, "git@bitbucket.com:org/repo", "main", metadataDir)
			require.ErrorContains(t, err, "unable to fetch references for repository")
		})

		it("uses the http proxy env vars", func() {
			require.NoError(t, os.Setenv("HTTPS_PROXY", "http://invalid-proxy"))
			defer os.Unsetenv("HTTPS_PROXY")

			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/basic", "master", metadataDir)
			require.Error(t, err)
			require.Contains(t, err.Error(), "proxyconnect tcp: dial tcp")
		})
	})
}

func isExecutableByAny(mode os.FileMode) bool {
	return mode&0111 != 0
}

func isExecutableByAll(mode os.FileMode) bool {
	return mode&0111 == 0111
}

type fakeGitKeychain struct{}

func (f fakeGitKeychain) Resolve(gitUrl string) (transport.AuthMethod, error) {
	return nil, nil
}
