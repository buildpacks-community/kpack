package blob

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/pivotal/kpack/pkg/util"
)

type Fetcher struct {
	Logger *log.Logger
}

func (f *Fetcher) Fetch(dir string, blobURL string) error {
	blob, err := url.Parse(blobURL)
	if err != nil {
		return err
	}

	resp, err := http.Get(blobURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(file.Name())

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	err = util.ExtractFile(file, dir)
	if err != nil {
		return err
	}
	f.Logger.Printf("Successfully downloaded %s in path %q", blob.Host+blob.Path, dir)
	return nil
}
