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
	spec.Run(t, "Test Git Checkout", testGitCheckout)
}

func testGitCheckout(t *testing.T, when spec.G, it spec.S) {
	when("#Fetch", func() {
		outputBuffer := &bytes.Buffer{}
		fetcher := Fetcher{
			Logger:   log.New(outputBuffer, "", 0),
			Keychain: fakeGitKeychain{},
		}
		var (
			testDir     string
			metadataDir string
		)

		it.Before(func() {
			var err error
			testDir, err = os.MkdirTemp("", "test-git")
			require.NoError(t, err)

			metadataDir, err = os.MkdirTemp("", "test-git")
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

				repository, err := gogit.PlainOpen(testDir)
				require.NoError(t, err)
				require.Contains(t, outputBuffer.String(), "Successfully cloned")

				p := path.Join(metadataDir, "project-metadata.toml")
				require.FileExists(t, p)

				var projectMetadata Project
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
			require.EqualError(t, err, "unable to fetch references for repository: couldn't find remote ref \"doesnotexist\"")
		})

		it("preserves symbolic links", func() {
			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/symlinks", "master", metadataDir)
			require.NoError(t, err)

			fileInfo, err := os.Lstat(path.Join(testDir, "bar"))
			require.NoError(t, err)
			require.Equal(t, fileInfo.Mode().Type(), os.ModeSymlink)
		})

		it("preserves executable permission", func() {
			err := fetcher.Fetch(testDir, "https://github.com/pivotal/kpack", "b8c0d491135595cc00ab78f6214bef8a7a20afd8", metadataDir)
			require.NoError(t, err)

			fileInfo, err := os.Lstat(path.Join(testDir, "hack", "local.sh"))
			require.NoError(t, err)
			require.True(t, isExecutableByAll(fileInfo.Mode()))

			fileInfo, err = os.Lstat(path.Join(testDir, "hack", "tools.go"))
			require.NoError(t, err)
			require.False(t, isExecutableByAny(fileInfo.Mode()))
		})

		it("records project-metadata.toml", func() {
			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/basic", "b029517f6300c2da0f4b651b8642506cd6aaf45d", metadataDir)
			require.NoError(t, err)

			p := path.Join(metadataDir, "project-metadata.toml")
			contents, err := os.ReadFile(p)
			require.NoError(t, err)

			expectedFile := `[source]
  type = "git"
  [source.metadata]
    repository = "https://github.com/git-fixtures/basic"
    revision = "b029517f6300c2da0f4b651b8642506cd6aaf45d"
  [source.version]
    commit = "b029517f6300c2da0f4b651b8642506cd6aaf45d"
`
			require.Equal(t, expectedFile, string(contents))
		})

		it("returns invalid credentials to fetch error on authentication required", func() {
			err := fetcher.Fetch(testDir, "git@bitbucket.com:org/repo", "main", metadataDir)
			require.ErrorContains(t, err, "unable to fetch references for repository")
		})

		it("initializes submodules", func() {
			fetcher.InitializeSubmodules = true
			err := fetcher.Fetch(testDir, "https://github.com/git-fixtures/submodule", "master", metadataDir)
			require.NoError(t, err)

			_, err = os.Lstat(path.Join(testDir, "basic", ".gitignore"))
			require.NoError(t, err)

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
