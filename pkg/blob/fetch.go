package blob

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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

	zipReader, err := zip.OpenReader(file.Name())
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		filePath := filepath.Join(dir, file.Name)
		if file.FileInfo().IsDir() {
			err := os.MkdirAll(filePath, file.Mode())
			if err != nil {
				return err
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, srcFile)

		outFile.Close()
		srcFile.Close()

		if err != nil {
			return err
		}
	}
	f.Logger.Printf("Successfully downloaded %s in path %q", blob.Host+blob.Path, dir)
	return nil
}
