package blob

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/BurntSushi/toml"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/archive"
)

var unexpectedBlobTypeError = errors.New("unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")

type Fetcher struct {
	Logger *log.Logger
}

func (f *Fetcher) Fetch(dir string, blobURL string, stripComponents int, metadataDir string) error {
	u, err := url.Parse(blobURL)
	if err != nil {
		return err
	}
	f.Logger.Printf("Downloading %s%s...", u.Host, u.Path)

	file, err := downloadBlob(blobURL)
	if err != nil {
		return err
	}
	defer os.RemoveAll(file.Name())

	mediaType, err := classifyFile(file)
	if err != nil {
		return err
	}

	checksum, err := sha256sum(file)
	if err != nil {
		return err
	}

	switch mediaType {
	case "application/zip":
		var info fs.FileInfo
		info, err = file.Stat()
		if err != nil {
			return err
		}
		err = archive.ExtractZip(file, info.Size(), dir, stripComponents)
	case "application/x-gzip":
		err = archive.ExtractTarGZ(file, dir, stripComponents)
	case "application/octet-stream":
		if !archive.IsTar(file.Name()) {
			return unexpectedBlobTypeError
		}
		err = archive.ExtractTar(file, dir, stripComponents)
	default:
		return unexpectedBlobTypeError
	}
	if err != nil {
		return err
	}

	projectMetadataFile, err := os.Create(path.Join(metadataDir, "project-metadata.toml"))
	if err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for blob: %s", metadataDir, blobURL)
	}
	defer projectMetadataFile.Close()

	projectMd := project{
		Source: source{
			Type: "blob",
			Metadata: metadata{
				Url: blobURL,
			},
			Version: version{
				SHA256: checksum,
			},
		},
	}
	if err := toml.NewEncoder(projectMetadataFile).Encode(projectMd); err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for blob: %s", metadataDir, blobURL)
	}

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

	file, err := os.CreateTemp("", "")
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

func sha256sum(reader io.ReadSeeker) (string, error) {
	hash := sha256.New()
	_, err := io.Copy(hash, reader)
	if err != nil {
		return "", err
	}

	_, err = reader.Seek(0, 0)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

type project struct {
	Source source `toml:"source"`
}

type source struct {
	Type     string   `toml:"type"`
	Metadata metadata `toml:"metadata"`
	Version  version  `toml:"version"`
}

type metadata struct {
	Url string `toml:"url"`
}

type version struct {
	SHA256 string `toml:"sha256sum"`
}
