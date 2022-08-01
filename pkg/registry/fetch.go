package registry

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/archive"
)

type contentType string

const (
	ContentTypeLabelKey string = "source.contenttype.kpack.io"

	zip   contentType = "zip"
	jar   contentType = "jar"
	war   contentType = "war"
	tar   contentType = "tar"
	targz contentType = "tar.gz"
)

type ImageClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
}

type Fetcher struct {
	Logger   *log.Logger
	Client   ImageClient
	Keychain authn.Keychain
}

func (f *Fetcher) Fetch(dir, registryImage string) error {
	f.Logger.Printf("Pulling %s...", registryImage)

	img, _, err := f.Client.Fetch(f.Keychain, registryImage)
	if err != nil {
		return err
	}

	cType, err := getContentType(img)
	if err != nil {
		return err
	}

	var handler func(img v1.Image, dir string) error
	switch cType {
	case zip, jar, war:
		handler = handleZip
	case tar:
		handler = handleTar
	case targz:
		handler = handleTarGZ
	default:
		handler = handleSource
	}

	if err := handler(img, dir); err != nil {
		return err
	}

	f.Logger.Printf("Successfully pulled %s in path %q", registryImage, dir)

	return nil
}

func getContentType(img v1.Image) (contentType, error) {
	cfg, err := img.ConfigFile()
	if err != nil {
		return "", err
	}

	if cfg.Config.Labels == nil {
		return "", nil
	}

	val, ok := cfg.Config.Labels[ContentTypeLabelKey]
	if !ok {
		return "", nil
	}

	return contentType(val), nil
}

func handleSource(img v1.Image, dir string) error {
	layers, err := img.Layers()
	if err != nil {
		return err
	}

	for _, layer := range layers {
		err := fetchLayer(layer, dir)
		if err != nil {
			return err
		}
	}

	return nil
}

func handleZip(img v1.Image, dir string) error {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	file, err := getSourceFile(img, tmpDir)
	if err != nil {
		return err
	}
	defer file.Close()

	if !archive.IsZip(file.Name()) {
		return errors.Errorf("expected file '%s' to be a zip archive", file.Name())
	}

	info, err := file.Stat()
	if err != nil {
		return err
	}

	return archive.ExtractZip(file, info.Size(), dir, 0)
}

func handleTar(img v1.Image, dir string) error {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	file, err := getSourceFile(img, tmpDir)
	if err != nil {
		return err
	}
	defer file.Close()

	if !archive.IsTar(file.Name()) {
		return errors.Errorf("expected file '%s' to be a tar archive", file.Name())
	}

	return archive.ExtractTar(file, dir, 0)
}

func handleTarGZ(img v1.Image, dir string) error {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	file, err := getSourceFile(img, tmpDir)
	if err != nil {
		return err
	}
	defer file.Close()

	return archive.ExtractTarGZ(file, dir, 0)
}

func getSourceFile(img v1.Image, dir string) (*os.File, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	if len(layers) != 1 {
		return nil, errors.Errorf("expected image to have exactly one layer")
	}

	err = fetchLayer(layers[0], dir)
	if err != nil {
		return nil, err
	}

	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	if len(infos) != 1 {
		return nil, errors.Errorf("expected layer to have exactly one file")
	}

	return os.Open(filepath.Join(dir, infos[0].Name()))
}

func fetchLayer(layer v1.Layer, dir string) error {
	reader, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer reader.Close()

	return archive.ExtractTar(reader, dir, 0)
}
