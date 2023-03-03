package git

import (
	"bytes"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"

	"github.com/sclevine/spec"
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

				fs := osfs.New(testDir)
				storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
				repository, err := gogit.Init(storage, fs)
				fmt.Println(outputBuffer.String())
				require.Contains(t, outputBuffer.String(), "Successfully cloned")
				branches, err := repository.Branches()
				require.NoError(t, err)
				branches.ForEach(func(branch *plumbing.Reference) error {
					fmt.Println("Branch name: ")
					fmt.Println(branch.Name())
					return nil
				})

				var projectMetadata project
				p := path.Join(metadataDir, "project-metadata.toml")
				md, err := toml.DecodeFile(p, &projectMetadata)
				for k := range md.Keys() {
					fmt.Println(k)
				}

				require.NoError(t, err)
				require.Equal(t, "git", projectMetadata.Source.Type)
				require.Equal(t, gitUrl, projectMetadata.Source.Metadata.Repository)
				require.Equal(t, revision, projectMetadata.Source.Metadata.Revision)

				refs, err := repository.References()
				refs.ForEach(func(r *plumbing.Reference) error {
					fmt.Println(r.Name())
					return nil
				})

				require.NoError(t, err)
				//require.Equal(t, ref.Hash(), projectMetadata.Source.Version.Commit)
			}
		}

		it("fetches remote HEAD", testFetch("https://github.com/git-fixtures/basic", "master"))
	})
}

type fakeGitKeychain struct {
}

type goGitFakeCredential struct {
	GoGitCredential
}

type fakeAuthMethod struct {
	transport.AuthMethod
}

func (c *goGitFakeCredential) Cred() (transport.AuthMethod, error) {
	// return fake transport.AuthMethod
	return &fakeAuthMethod{}, nil
}

func (f fakeGitKeychain) Resolve(url string, usernameFromUrl string, allowedTypes CredentialType) (GoGitCredential, error) {
	return &goGitFakeCredential{}, nil
	//return nil, errors.New("no auth available")
}
