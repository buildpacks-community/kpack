package blob

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
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

	// file, err := downloadBlob(blobURL)

	// TODO: Here use regex to figure out blob, I think the properties should be configured in cnbimage source.blob.xxx
	url, container, name, err := extractBlobInfo(blobURL)
	if err != nil {
		return err
	}
	credential, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		return err
	}
	client, err := azblob.NewClient(url, credential, nil)
	if err != nil {
		return err
	}
	file, err := downloadAzureBlob(*client, container, name)

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
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for blob: %s", metadataDir, blobURL)
	}

	f.Logger.Printf("Successfully downloaded %s%s in path %q", u.Host, u.Path, dir)

	return nil
}

func extractBlobInfo(blobURL string) (string, string, string, error) {
	// Define the regex pattern
	pattern := `^(https:\/\/[a-zA-Z0-9-]+\.blob\.core\.windows\.net)\/([^\/]+)\/(.+)$`

	// Compile the regex pattern
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return "", "", "", err
	}

	// Find matches in the blob URL
	matches := regex.FindStringSubmatch(blobURL)
	if matches == nil || len(matches) != 4 {
		return "", "", "", fmt.Errorf("unable to extract blob info from URL")
	}

	// Extract the host, container name, and blob name
	host := matches[1]
	containerName := matches[2]
	blobName := matches[3]

	return host, containerName, blobName, nil
}

func downloadAzureBlob(client azblob.Client, containerName string, blobName string) (*os.File, error) {
	get, err := client.DownloadStream(context.TODO(), containerName, blobName, nil)
	if err != nil {
		fmt.Println("Failed to download blob:", err)
		return nil, err
	}

	defer get.Body.Close()
	// Create a file to write the blob content
	file, err := os.CreateTemp("", "")
	if err != nil {
		fmt.Println("Failed to create file:", err)
		return nil, err
	}

	// Copy blob content to local file
	_, err = io.Copy(file, get.Body)
	if err != nil {
		fmt.Println("Failed to copy blob content to file:", err)
		return nil, err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	return file, nil
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
