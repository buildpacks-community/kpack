package main

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

	"github.com/pivotal/kpack/pkg/dockercreds"
)

func fetchImage(dir string, logger *log.Logger) {
	imagePullSecrets, err := dockercreds.ParseDockerPullSecrets("/imagePullSecrets")
	if err != nil {
		log.Fatal(err)
	}

	ref, err := name.ParseReference(*registryImage, name.WeakValidation)
	if err != nil {
		logger.Fatal(err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.NewMultiKeychain(imagePullSecrets, authn.DefaultKeychain)))
	if err != nil {
		logger.Fatal(err)
	}

	layers, err := img.Layers()
	if err != nil {
		logger.Fatal(err)
	}

	for _, layer := range layers {
		fetchLayer(layer, logger, dir)
	}
	logger.Printf("Successfully pulled %s in path %q", *registryImage, dir)
}

func fetchLayer(layer v1.Layer, logger *log.Logger, dir string) {
	reader, err := layer.Uncompressed()
	if err != nil {
		logger.Fatal(err)
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
				logger.Fatal(err.Error())
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			logger.Fatal(err.Error())
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
		if err != nil {
			logger.Fatal(err.Error())
		}

		_, err = io.Copy(outFile, tarReader)
		outFile.Close()
		if err != nil {
			logger.Fatal(err.Error())
		}
	}
}
