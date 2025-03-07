package blob_test

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
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
		dir         string
		metadataDir string
	)

	it.Before(func() {
		var err error
		dir, err = os.MkdirTemp("", "fetch_test")
		require.NoError(t, err)

		metadataDir, err = os.MkdirTemp("", "fetch_test")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(dir))
	})

	for _, f := range []string{"test.zip", "test.tar", "test.tar.gz"} {
		testFile := f
		it("unpacks "+testFile, func() {
			err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, testFile), 0, metadataDir)
			require.NoError(t, err)

			files, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, files, 1)
			testDir := files[0]
			require.Equal(t, "testdir", testDir.Name())
			require.True(t, testDir.IsDir())

			files, err = os.ReadDir(filepath.Join(dir, testDir.Name()))
			require.NoError(t, err)
			require.Len(t, files, 1)

			testFile := files[0]
			require.Equal(t, "testfile", testFile.Name())
			require.False(t, testFile.IsDir())
			file, err := os.ReadFile(filepath.Join(dir, testDir.Name(), testFile.Name()))
			require.NoError(t, err)
			require.Equal(t, "test file contents", string(file))

			require.Contains(t, output.String(), "Successfully downloaded")
		})
	}

	it("unpacks files with mode 0777 when files are compressed in fat (MSDOS) format", func() {
		// Set no umask to test file mode
		oldMask := syscall.Umask(0)
		defer syscall.Umask(oldMask)
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "fat-zip.zip"), 0, metadataDir)
		require.NoError(t, err)

		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, files, 1)

		testFile := files[0]
		require.Equal(t, "some-file.txt", testFile.Name())
		info, err := testFile.Info()
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0777).String(), info.Mode().String())

		require.Contains(t, output.String(), "Successfully downloaded")
	})

	it("sets the correct file mode", func() {
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test-exe.tar"), 0, metadataDir)
		require.NoError(t, err)

		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, files, 1)

		testDir := files[0]
		require.Equal(t, "test-exe", testDir.Name())
		require.True(t, testDir.IsDir())

		files, err = os.ReadDir(filepath.Join(dir, testDir.Name()))
		require.NoError(t, err)
		require.Len(t, files, 1)

		testFile := files[0]
		require.Equal(t, "runnable", testFile.Name())
		require.False(t, testFile.IsDir())

		info, err := os.Stat(filepath.Join(dir, testDir.Name(), testFile.Name()))
		require.NoError(t, err)
		require.Equal(t, 0755, int(info.Mode()))
	})

	it("records project-metadata.toml", func() {
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.tar"), 0, metadataDir)
		require.NoError(t, err)

		p := path.Join(metadataDir, "project-metadata.toml")
		contents, err := os.ReadFile(p)
		require.NoError(t, err)

		expectedFile := fmt.Sprintf(`[source]
  type = "blob"
  [source.metadata]
    url = "%v/test.tar"
  [source.version]
    sha256sum = "e54f870c2d76e5a1e577b9ff6c8c56f42b539fff83cf86cccf6b16ce6e177a4e"
`, server.URL)

		require.Equal(t, expectedFile, string(contents))
	})
	for _, archiveFile := range []string{"parent.tar", "parent.tar.gz", "parent.zip"} {
		it("strips parent components from "+archiveFile, func() {
			err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, archiveFile), 1, metadataDir)
			require.NoError(t, err)

			files, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Len(t, files, 1)

			testDir := files[0]
			require.Equal(t, "child.txt", testDir.Name())
			require.False(t, testDir.IsDir())
		})
	}

	it("errors when url is inaccessible", func() {
		url := fmt.Sprintf("%s/%s", server.URL, "invalid.zip")
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "invalid.zip"), 0, metadataDir)
		require.EqualError(t, err, fmt.Sprintf("failed to get blob %s: 404 Not Found: 404 page not found\n", url))
	})

	it("errors when the blob file type is unexpected", func() {
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.txt"), 0, metadataDir)
		require.EqualError(t, err, "unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")
	})

	it("errors when the blob content type is unexpected", func() {
		err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.html"), 0, metadataDir)
		require.EqualError(t, err, "unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")
	})

	when("there's auth required", func() {
		var (
			handler = &authHandler{http.FileServer(http.Dir("./testdata")), nil}
			server  = httptest.NewServer(handler)
		)

		it("doesn't send headers when there's no keychain", func() {
			err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.zip"), 0, metadataDir)
			require.NoError(t, err)

			require.NotContains(t, handler.headers, "Authorization")
		})

		it("uses the auth and headers from the keychain", func() {
			fetcher = &blob.Fetcher{
				Logger:   log.New(output, "", 0),
				Keychain: &fakeKeychain{"some-auth", map[string]string{"Some-Header": "some-value"}, nil},
			}

			err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.zip"), 0, metadataDir)
			require.NoError(t, err)
			headers := handler.headers

			require.Contains(t, headers, "Authorization")
			require.Equal(t, []string{"some-auth"}, headers["Authorization"])

			require.Contains(t, headers, "Some-Header")
			require.Equal(t, []string{"some-value"}, headers["Some-Header"])
		})

		it("surfaces the error", func() {
			fetcher = &blob.Fetcher{
				Logger:   log.New(output, "", 0),
				Keychain: &fakeKeychain{"", nil, fmt.Errorf("some-error")},
			}

			err := fetcher.Fetch(dir, fmt.Sprintf("%s/%s", server.URL, "test.zip"), 0, metadataDir)
			require.EqualError(t, err, "failed to resolve creds: some-error")
		})
	})
}

type authHandler struct {
	h       http.Handler
	headers http.Header
}

func (a *authHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.headers = r.Header.Clone()
	a.h.ServeHTTP(w, r)
}
