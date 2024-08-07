package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"reflect"
	"regexp"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/imgutil/layer"
	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	lifecycleMetadataLabel = "io.buildpacks.lifecycle.metadata"
	lifecycleLocation      = "/cnb/lifecycle/"
	lifecycleVersion       = "0.20.0"
)

var (
	tag = flag.String("tag", "", "tag for lifecycle image")

	normalizedTime = time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)
)

func main() {
	flag.Parse()

	image, err := lifecycleImage(
		fmt.Sprintf("https://github.com/buildpacks/lifecycle/releases/download/v%s/lifecycle-v%s+linux.x86-64.tgz", lifecycleVersion, lifecycleVersion),
		fmt.Sprintf("https://github.com/buildpacks/lifecycle/releases/download/v%s/lifecycle-v%s+windows.x86-64.tgz", lifecycleVersion, lifecycleVersion),
	)
	if err != nil {
		log.Fatal(err)
	}

	identifier, err := (&registry.Client{}).Save(authn.DefaultKeychain, *tag, image)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(fmt.Sprintf("saved lifecycle image: %s ", identifier))

}

func lifecycleImage(linuxUrl, windowsUrl string) (v1.Image, error) {
	image, err := random.Image(0, 0)
	if err != nil {
		return nil, err
	}

	linuxDescriptor, err := lifecycleDescriptor(linuxUrl)
	if err != nil {
		return nil, err
	}
	windowsDescriptor, err := lifecycleDescriptor(windowsUrl)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(linuxDescriptor, windowsDescriptor) {
		return nil, errors.New("linux and windows lifecycle descriptors do not match. Check urls.")
	}

	linuxLayer, err := lifecycleLayer(linuxUrl, "linux")
	if err != nil {
		return nil, err
	}
	linuxDiffID, err := linuxLayer.DiffID()
	if err != nil {
		return nil, err
	}

	image, err = imagehelpers.SetStringLabel(image, "linux", linuxDiffID.String())
	if err != nil {
		return nil, err
	}

	windowsLayer, err := lifecycleLayer(windowsUrl, "windows")
	if err != nil {
		return nil, err
	}

	windowsDiffID, err := windowsLayer.DiffID()
	if err != nil {
		return nil, err
	}

	image, err = imagehelpers.SetStringLabel(image, "windows", windowsDiffID.String())
	if err != nil {
		return nil, err
	}

	image, err = mutate.AppendLayers(image, linuxLayer, windowsLayer)
	if err != nil {
		return nil, err
	}

	return imagehelpers.SetLabels(image, map[string]interface{}{
		lifecycleMetadataLabel: lifecycleDescriptorToMetadata(linuxDescriptor),
	})
}

func lifecycleDescriptor(url string) (cnb.LifecycleDescriptor, error) {
	lr, err := lifecycleReader(url)
	if err != nil {
		return cnb.LifecycleDescriptor{}, err
	}
	defer lr.Close()
	tr := tar.NewReader(lr)
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}

		name := header.Name
		if name == "lifecycle.toml" {
			descriptor := cnb.LifecycleDescriptor{}
			if _, err := toml.NewDecoder(tr).Decode(&descriptor); err != nil {
				return cnb.LifecycleDescriptor{}, err
			}
			return descriptor, nil
		}

		continue
	}
	return cnb.LifecycleDescriptor{}, errors.New("could not find lifecycle descriptor lifecyle.toml")
}

func lifecycleDescriptorToMetadata(descriptor cnb.LifecycleDescriptor) cnb.LifecycleMetadata {
	return cnb.LifecycleMetadata{
		LifecycleInfo: descriptor.Info,
		API:           descriptor.API,
		APIs:          descriptor.APIs,
	}
}

func lifecycleLayer(url, os string) (v1.Layer, error) {
	b := &bytes.Buffer{}
	tw := newLayerWriter(b, os)

	err := tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     lifecycleLocation,
		Mode:     0755,
		ModTime:  normalizedTime,
	})
	if err != nil {
		return nil, err
	}

	var regex = regexp.MustCompile(`^[^/]+/([^/]+)$`)

	lr, err := lifecycleReader(url)
	if err != nil {
		return nil, err
	}
	defer lr.Close()
	tr := tar.NewReader(lr)
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}

		pathMatches := regex.FindStringSubmatch(path.Clean(header.Name))
		if pathMatches != nil {
			binaryName := pathMatches[1]

			header.Name = lifecycleLocation + binaryName
			header.ModTime = normalizedTime
			err = tw.WriteHeader(header)
			if err != nil {
				return nil, err
			}

			buf, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}

			_, err = tw.Write(buf)
			if err != nil {
				return nil, err
			}
		}

	}

	return tarball.LayerFromReader(b)
}

func lifecycleReader(url string) (io.ReadCloser, error) {
	dir, err := os.MkdirTemp("", "lifecycle")
	if err != nil {
		return nil, err
	}

	lifecycleLocation := dir + "/lifecycle.tgz"

	err = download(lifecycleLocation, url)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(lifecycleLocation)
	if err != nil {
		return nil, err
	}

	gzr, err := gzip.NewReader(file)

	return &ReadCloserWrapper{
		Reader: gzr,
		closer: func() error {
			defer os.RemoveAll(dir)
			defer file.Close()
			return gzr.Close()
		},
	}, err
}

func download(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// copied from github.com/docker/docker/pkg/ioutils
type ReadCloserWrapper struct {
	io.Reader
	closer func() error
}

func (r *ReadCloserWrapper) Close() error {
	return r.closer()
}

func newLayerWriter(fileWriter io.Writer, os string) layerWriter {
	if os == "windows" {
		return layer.NewWindowsWriter(fileWriter)
	}
	return tar.NewWriter(fileWriter)
}

type layerWriter interface {
	WriteHeader(hdr *tar.Header) error
	Write(b []byte) (int, error)
	Close() error
}
