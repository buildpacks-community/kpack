package dir

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/pkg/errors"
)

var unexpectedBlobTypeError = errors.New("unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")

type Fetcher struct {
	Logger *log.Logger
}

func (f *Fetcher) Fetch(appDir string, dirPath string) error {
	// Copy dirPath content to appDir
	f.Logger.Printf("Successfully downloaded %s%s in path %q", u.Host, u.Path, dir)

	return nil
}

func downloadBlob(blobURL string) (*os.File, error) {
	resp, err := http.Get(blobURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to get blob %s", blobURL)
	}

	file, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func classifyFile(reader io.ReadSeeker) (string, error) {
	buf := make([]byte, 512)
	_, err := reader.Read(buf)
	if err != nil {
		return "", err
	}

	_, err = reader.Seek(0, 0)
	if err != nil {
		return "", err
	}

	return http.DetectContentType(buf), nil
}
