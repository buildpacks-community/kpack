package s3

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type Fetcher struct {
	Logger *log.Logger
}

func (f *Fetcher) Fetch(dir string, s3URL string, s3AccessKey string, s3SecretKey string, s3Bucket string, s3File string, s3ForcePathStyle bool, s3Region string) error {
	blob, err := url.Parse(s3URL)
	if err != nil {
		return err
	}

	option := session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}

	region := s3Region
	if region == "" {
		region = "s3RegionStub" // Needed by amazon-sdk
	}

	option.Config = aws.Config{
		Endpoint:         aws.String(s3URL),
		S3ForcePathStyle: aws.Bool(s3ForcePathStyle),
		Credentials:      credentials.NewStaticCredentials(s3AccessKey, s3SecretKey, ""),
		Region:           aws.String(region),
	}

	sess, err := session.NewSessionWithOptions(option)
	downloader := s3manager.NewDownloader(sess)

	file, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(file.Name())

	_, err = downloader.Download(file, &s3.GetObjectInput{
		Bucket: aws.String(s3Bucket),
		Key:    aws.String(s3File),
	})

	if err != nil {
		return err
	}

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
		err = extractTar(r, dir)
	}

	if err != nil {
		return err
	}
	f.Logger.Printf("Successfully downloaded %s/%s/%s in path %q", blob.Host+blob.Path, s3Bucket, s3File, dir)
	return nil
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
			_, err = io.Copy(outFile, tarReader)
			if err != nil {
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
