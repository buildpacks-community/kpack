package main

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

func downloadBlob(dir string, logger *log.Logger) {
	blob, err := url.Parse(*blobURL)
	if err != nil {
		logger.Fatal(err.Error())
	}

	resp, err := http.Get(*blobURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer resp.Body.Close()

	file, err := ioutil.TempFile("", "")
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer os.RemoveAll(file.Name())

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		logger.Fatal(err.Error())
	}

	zipReader, err := zip.OpenReader(file.Name())
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		filePath := filepath.Join(dir, file.Name)
		if file.FileInfo().IsDir() {
			err := os.MkdirAll(filePath, file.Mode())
			if err != nil {
				logger.Fatal(err.Error())
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			logger.Fatal(err.Error())
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			logger.Fatal(err.Error())
		}

		srcFile, err := file.Open()
		if err != nil {
			logger.Fatal(err.Error())
		}

		_, err = io.Copy(outFile, srcFile)

		outFile.Close()
		srcFile.Close()

		if err != nil {
			logger.Fatal(err.Error())
		}
	}
	logger.Printf("Successfully downloaded %s in path %q", blob.Host+blob.Path, dir)
}
