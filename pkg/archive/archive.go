package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func IsTar(fileName string) bool {
	file, err := os.Open(fileName)
	if err != nil {
		return false
	}
	defer file.Close()

	_, err = tar.NewReader(file).Next()
	if err != nil {
		return false
	}

	return true
}

func ExtractTar(reader io.Reader, dir string, stripComponents int) error {
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		strippedFileName := stripPath(header.Name, stripComponents)
		if strippedFileName == "" {
			continue
		}

		filePath := filepath.Join(dir, strippedFileName)
		switch header.Typeflag {
		case tar.TypeDir:
			err := os.MkdirAll(filePath, header.FileInfo().Mode())
			if err != nil {
				return err
			}
		case tar.TypeReg:
			if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
				return err
			}

			outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}

			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func ExtractTarGZ(reader io.Reader, dir string, stripComponents int) error {
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}

	return ExtractTar(gzr, dir, stripComponents)
}

func IsZip(fileName string) bool {
	file, err := os.Open(fileName)
	if err != nil {
		return false
	}
	defer file.Close()

	// http://golang.org/pkg/net/http/#DetectContentType
	buf := make([]byte, 512)
	_, err = file.Read(buf)
	if err != nil {
		return false
	}

	return http.DetectContentType(buf) == "application/zip"
}

func ExtractZip(reader io.ReaderAt, size int64, dir string, stripComponents int) error {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return err
	}

	for _, file := range zipReader.File {

		strippedFileName := stripPath(file.Name, stripComponents)
		if strippedFileName == "" {
			continue
		}

		filePath := filepath.Join(dir, strippedFileName)
		fileMode := file.Mode()
		if isFatFile(file.FileHeader) {
			fileMode = 0777
		}

		if file.FileInfo().IsDir() {
			err := os.MkdirAll(filePath, fileMode)
			if err != nil {
				return err
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
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

		if err := outFile.Close(); err != nil {
			return err
		}

		if err := srcFile.Close(); err != nil {
			return err
		}
	}
	return nil
}

func isFatFile(header zip.FileHeader) bool {
	var (
		creatorFAT  uint16 = 0
		creatorVFAT uint16 = 14
	)

	// This identifies FAT files, based on the `zip` source: https://golang.org/src/archive/zip/struct.go
	firstByte := header.CreatorVersion >> 8
	return firstByte == creatorFAT || firstByte == creatorVFAT
}

func stripPath(source string, stripComponents int) string {
	components := strings.Split(source, string(filepath.Separator))

	if len(components) <= stripComponents {
		return ""
	}

	return filepath.Join(components[stripComponents:]...)
}
