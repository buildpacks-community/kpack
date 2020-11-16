package registry_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestRegistrySourceFetcher(t *testing.T) {
	spec.Run(t, "testRegistrySourceFetcher", testRegistrySourceFetcher)
}

func testRegistrySourceFetcher(t *testing.T, when spec.G, it spec.S) {
	var (
		keychain = &registryfakes.FakeKeychain{}
		client   = registryfakes.NewFakeClient()
		output   = &bytes.Buffer{}
		fetcher  = &registry.Fetcher{
			Logger:   log.New(output, "", 0),
			Client:   client,
			Keychain: keychain,
		}
		dir string
	)

	it.Before(func() {
		var err error
		dir, err = ioutil.TempDir("", "")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(dir))
	})

	testCases := []string{"zip.tar", "zip", "tar.tar", "tar", "targz.tar", "tar.gz"}
	for i := 0; i+1 < len(testCases); i += 2 {
		testFile := testCases[i]
		testContentType := testCases[i+1]

		it("handles unexploded file types: "+testFile, func() {
			buf, err := ioutil.ReadFile(filepath.Join("testdata", testFile))
			require.NoError(t, err)

			img := createSourceImage(t, buf, testContentType)

			repoName := fmt.Sprintf("registry.example/some-image-%d", time.Now().Second())
			client.AddImage(repoName, img, keychain)

			err = fetcher.Fetch(dir, repoName)
			require.NoError(t, err)

			files, err := ioutil.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, files, 1)

			testDir := files[0]
			require.Equal(t, "testdir", testDir.Name())
			require.True(t, testDir.IsDir())

			files, err = ioutil.ReadDir(filepath.Join(dir, testDir.Name()))
			require.NoError(t, err)
			require.Len(t, files, 1)

			testFile := files[0]
			require.Equal(t, "testfile", testFile.Name())
			require.False(t, testFile.IsDir())

			file, err := ioutil.ReadFile(filepath.Join(dir, testDir.Name(), testFile.Name()))
			require.NoError(t, err)
			require.Equal(t, "test file contents", string(file))

			require.Contains(t, output.String(), "Successfully pulled")
		})
	}

	it("handles regular source images", func() {
		buf, err := ioutil.ReadFile(filepath.Join("testdata", "reg.tar"))
		require.NoError(t, err)

		img := createSourceImage(t, buf, "")

		repoName := fmt.Sprintf("registry.example/some-image-%d", time.Now().Second())
		client.AddImage(repoName, img, keychain)

		err = fetcher.Fetch(dir, repoName)
		require.NoError(t, err)

		files, err := ioutil.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, files, 1)

		testFile := files[0]
		require.Equal(t, "test.txt", testFile.Name())
		require.False(t, testFile.IsDir())

		file, err := ioutil.ReadFile(filepath.Join(dir, testFile.Name()))
		require.NoError(t, err)
		require.Equal(t, "test file contents\n", string(file))

		require.Contains(t, output.String(), "Successfully pulled")
	})

	it("handles directories with improper headers", func() {
		buf, err := ioutil.ReadFile(filepath.Join("testdata", "layer.tar"))
		require.NoError(t, err)

		img := createSourceImage(t, buf, "")

		repoName := "registry.example/test-exe"
		client.AddImage(repoName, img, keychain)

		err = fetcher.Fetch(dir, repoName)
		require.NoError(t, err)

		// the vendor/cache directory doesnt have proper headers
		_, err = ioutil.ReadFile(filepath.Join(dir, "/vendor/cache/diff-lcs-1.4.2.gem"))
		require.NoError(t, err)

		require.Contains(t, output.String(), "Successfully pulled")
	})

	it("sets the correct file mode", func() {
		buf, err := ioutil.ReadFile(filepath.Join("testdata", "tarexe.tar"))
		require.NoError(t, err)

		img := createSourceImage(t, buf, "tar")

		repoName := "registry.example/test-exe"
		client.AddImage(repoName, img, keychain)

		err = fetcher.Fetch(dir, repoName)
		require.NoError(t, err)

		files, err := ioutil.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, files, 1)

		testDir := files[0]
		require.Equal(t, "test-exe", testDir.Name())
		require.True(t, testDir.IsDir())

		files, err = ioutil.ReadDir(filepath.Join(dir, testDir.Name()))
		require.NoError(t, err)
		require.Len(t, files, 1)

		testFile := files[0]
		require.Equal(t, "runnable", testFile.Name())
		require.False(t, testFile.IsDir())

		info, err := os.Stat(filepath.Join(dir, testDir.Name(), testFile.Name()))
		require.NoError(t, err)
		require.Equal(t, 0755, int(info.Mode()))
	})

	it("errors when the registry is inaccessible", func() {
		registryError := errors.New("some registry error")
		client.SetFetchError(registryError)

		err := fetcher.Fetch(dir, "registry.example/error")
		require.Equal(t, err, registryError)
	})
}

func createSourceImage(t *testing.T, buf []byte, contentType string) v1.Image {
	img, err := random.Image(0, 0)
	require.NoError(t, err)

	layerReader, err := tarball.LayerFromReader(bytes.NewBuffer(buf))
	require.NoError(t, err)

	img, err = mutate.AppendLayers(img, layerReader)
	require.NoError(t, err)

	img, err = imagehelpers.SetStringLabel(img, registry.ContentTypeLabelKey, contentType)
	require.NoError(t, err)

	return img
}
