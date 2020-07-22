package util

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

var errUnexpectedBlobType error = errors.New("unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")

// ExtractFile Extracts a compressed file into a provided directory
func ExtractFile(file *os.File, dir string) error {
	fileType, err := classifyFile(file.Name())
	if err != nil {
		return err
	}

	switch fileType {
	case "application/zip":
		err = extractZip(file, dir)
	case "application/x-gzip":
		r, err := os.Open(file.Name())
		if err != nil {
			return err
		}

		gzf, err := gzip.NewReader(r)
		if err != nil {
			return err
		}

		err = extractTar(gzf, dir)
	case "application/octet-stream":
		r, err := os.Open(file.Name())
		if err != nil {
			return err
		}

		if !isTar(r) {
			return errUnexpectedBlobType
		}

		if _, err := r.Seek(0, 0); err != nil {
			return err
		}

		err = extractTar(r, dir)
	default:
		return errUnexpectedBlobType
	}

	if err != nil {
		return err
	}
	return nil
}

func isTar(reader io.Reader) bool {
	tr := tar.NewReader(reader)
	_, err := tr.Next()
	return err == nil
}

func extractTar(reader io.Reader, dir string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		filePath := filepath.Join(dir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			err := os.MkdirAll(filePath, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(filePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractZip(file *os.File, dir string) error {
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

		if _, err := io.Copy(outFile, srcFile); err != nil {
			return err
		}

		outFile.Close()
		srcFile.Close()
	}
	return nil
}

func classifyFile(f string) (string, error) {
	file, err := os.Open(f)
	if err != nil {
		return "", err
	}

	defer file.Close()

	buff := make([]byte, 512)
	_, err = file.Read(buff)

	if err != nil {
		return "", err
	}

	return http.DetectContentType(buff), nil
}
