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

	"github.com/pivotal/kpack/pkg/archive"
)

var errUnexpectedBlobType = fmt.Errorf("unexpected blob file type, must be one of .zip, .tar.gz, .tar, .jar")

type Fetcher struct {
	Logger   *log.Logger
	Keychain Keychain
}

func (f *Fetcher) Fetch(dir string, blobURL string, stripComponents int, metadataDir string) error {
	u, err := url.Parse(blobURL)
	if err != nil {
		return err
	}

	var headers map[string]string
	if f.Keychain != nil {
		var auth string
		auth, headers, err = f.Keychain.Resolve(blobURL)
		if err != nil {
			return fmt.Errorf("failed to resolve creds: %v", err)
		}

		if headers == nil {
			headers = make(map[string]string)
		}
		headers["Authorization"] = auth
	}

	f.Logger.Printf("Downloading %s%s...", u.Host, u.Path)

	file, err := downloadBlob(blobURL, headers)
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
			return errUnexpectedBlobType
		}
		err = archive.ExtractTar(file, dir, stripComponents)
	default:
		return errUnexpectedBlobType
	}
	if err != nil {
		return err
	}

	projectMetadataFile, err := os.Create(path.Join(metadataDir, "project-metadata.toml"))
	if err != nil {
		return fmt.Errorf("invalid metadata destination '%s/project-metadata.toml' for blob '%s': %v", metadataDir, blobURL, err)
	}
	defer projectMetadataFile.Close()

	projectMd := Project{
		Source: Source{
			Type: "blob",
			Metadata: Metadata{
				Url: blobURL,
			},
			Version: Version{
				SHA256: checksum,
			},
		},
	}
	if err := toml.NewEncoder(projectMetadataFile).Encode(projectMd); err != nil {
		return fmt.Errorf("invalid metadata destination '%s/project-metadata.toml' for blob '%s': %v", metadataDir, blobURL, err)
	}

	f.Logger.Printf("Successfully downloaded %s%s in path %q", u.Host, u.Path, dir)

	return nil
}

func downloadBlob(blobURL string, headers map[string]string) (*os.File, error) {
	req, err := http.NewRequest(http.MethodGet, blobURL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var body []byte
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to get blob %s: %d %s", blobURL, resp.StatusCode, http.StatusText(resp.StatusCode))
		}

		return nil, fmt.Errorf("failed to get blob %s: %d %s: %s", blobURL, resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
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

type Project struct {
	Source Source `toml:"source"`
}

type Source struct {
	Type     string   `toml:"type"`
	Metadata Metadata `toml:"metadata"`
	Version  Version  `toml:"version"`
}

type Metadata struct {
	Url string `toml:"url"`
}

type Version struct {
	SHA256 string `toml:"sha256sum"`
}
