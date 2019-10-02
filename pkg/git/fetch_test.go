package git_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"

	"github.com/pivotal/kpack/pkg/git"
)

func TestGitCheckout(t *testing.T) {
	spec.Run(t, "Test Describe Image", testGitCheckout)
}

func testGitCheckout(t *testing.T, when spec.G, it spec.S) {
	when("#Fetch", func() {
		outpuBuffer := &bytes.Buffer{}
		fetcher := git.Fetcher{
			Logger:   log.New(outpuBuffer, "", 0),
			Keychain: fakeGitKeychain{},
		}
		var testDir string
		it.Before(func() {
			var err error
			testDir, err = ioutil.TempDir("", "test-git")
			require.NoError(t, err)
		})

		it.After(func() {
			require.NoError(t, os.RemoveAll(testDir))
		})

		testFetch := func(gitUrl, revision string) func() {
			return func() {
				err := fetcher.Fetch(testDir, gitUrl, revision)
				require.NoError(t, err)

				repository, err := gogit.PlainOpenWithOptions(testDir, &gogit.PlainOpenOptions{})
				require.NoError(t, err)

				worktree, err := repository.Worktree()
				require.NoError(t, err)

				status, err := worktree.Status()
				require.NoError(t, err)

				require.True(t, status.IsClean())

				require.Contains(t, outpuBuffer.String(), fmt.Sprintf("Successfully cloned \"%s\" @ \"%s\"", gitUrl, revision))
			}
		}

		it("fetches remote HEAD", testFetch("https://github.com/git-fixtures/basic", "master"))

		it("fetches a branch", testFetch("https://github.com/git-fixtures/basic", "branch"))

		it("fetches a tag", testFetch("https://github.com/git-fixtures/tags", "lightweight-tag"))

		it("fetches a revision", testFetch("https://github.com/git-fixtures/basic", "b029517f6300c2da0f4b651b8642506cd6aaf45d"))
	})
}

type fakeGitKeychain struct{}

func (f fakeGitKeychain) Resolve(gitUrl string) (transport.AuthMethod, error) {
	return nil, nil
}
