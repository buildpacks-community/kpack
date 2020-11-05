package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func ExtractTar(reader io.Reader, dir string) error {
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

func ExtractTarGZ(reader io.Reader, dir string) error {
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}

	return ExtractTar(gzr, dir)
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

func ExtractZip(reader io.ReaderAt, size int64, dir string) error {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return err
	}

	for _, file := range zipReader.File {
		filePath := filepath.Join(dir, file.Name)
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
