package registry

import (
	"archive/tar"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Fetcher struct {
	Logger   *log.Logger
	Keychain authn.Keychain
}

func (f *Fetcher) Fetch(dir, registryImage string) error {
	ref, err := name.ParseReference(registryImage, name.WeakValidation)
	if err != nil {
		return err
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(f.Keychain))
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return err
	}

	for _, layer := range layers {
		err := fetchLayer(layer, f.Logger, dir)
		if err != nil {
			return err
		}
	}
	f.Logger.Printf("Successfully pulled %s in path %q", registryImage, dir)
	return nil
}

func fetchLayer(layer v1.Layer, logger *log.Logger, dir string) error {
	reader, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Fatal(err)
		}

		filePath := filepath.Join(dir, header.Name)
		if header.FileInfo().IsDir() {
			err := os.MkdirAll(filePath, header.FileInfo().Mode())
			if err != nil {
				return err
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, tarReader)
		outFile.Close()
		if err != nil {
			return err

		}
	}
	return nil
}
