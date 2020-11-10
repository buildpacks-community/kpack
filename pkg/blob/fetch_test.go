package blob_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/blob"
)

func TestBlobFetcher(t *testing.T) {
	spec.Run(t, "testBlobFetcher", testBlobFetcher)
}

func testBlobFetcher(t *testing.T, when spec.G, it spec.S) {
	var (
		handler = http.FileServer(http.Dir("./testdata"))
		server  = httptest.NewServer(handler)
		output  = &bytes.Buffer{}
		fetcher = &blob.Fetcher{
			Logger: log.New(output, "", 0),
		}
		dir string
	)

	it.Before(func() {
		var err error
		dir, err = ioutil.TempDir("", "fetch_test")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(dir))
	})

	for _, f := range []string{"test.zip", "test.tar", "test.tar.gz"} {
		testFile := f
		it("unpacks "+testFile, func() {
			err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, testFile))
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

			require.Contains(t, output.String(), "Successfully downloaded")
		})
	}

	it("unpacks files with mode 0777 when files are compressed in fat (MSDOS) format", func() {
		// Set no umask to test file mode
		oldMask := syscall.Umask(0)
		defer syscall.Umask(oldMask)
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "fat-zip.zip"))
		require.NoError(t, err)

		files, err := ioutil.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, files, 1)

		testFile := files[0]
		require.Equal(t, "some-file.txt", testFile.Name())

		require.Equal(t, os.FileMode(0777).String(), testFile.Mode().String())

		require.Contains(t, output.String(), "Successfully downloaded")
	})

	it("sets the correct file mode", func() {
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test-exe.tar"))
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

	it("errors when url is inaccessible", func() {
		url := fmt.Sprintf("%s/%s", server.URL, "invalid.zip")
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "invalid.zip"))
		require.EqualError(t, err, fmt.Sprintf("failed to get blob %s", url))
	})

	it("errors when the blob file type is unexpected", func() {
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.txt"))
		require.EqualError(t, err, "unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")
	})

	it("errors when the blob content type is unexpected", func() {
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.html"))
		require.EqualError(t, err, "unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")
	})
}
