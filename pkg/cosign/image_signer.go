package cosign

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/buildpacks/lifecycle"
	"github.com/sigstore/cosign/cmd/cosign/cli"
)

type SignFunc func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error

type ImageSigner struct {
	Logger   *log.Logger
	signFunc SignFunc
}

const (
	cosignRepositoryEnv       = "COSIGN_REPOSITORY"
	cosignDockerMediaTypesEnv = "COSIGN_DOCKER_MEDIA_TYPES"
)

func NewImageSigner(logger *log.Logger, signFunc SignFunc) *ImageSigner {
	return &ImageSigner{
		Logger:   logger,
		signFunc: signFunc,
	}
}

func (s *ImageSigner) Sign(ctx context.Context, report lifecycle.ExportReport, secretLocation string, annotations, cosignRepositories, cosignDockerMediaTypes map[string]interface{}) error {
	cosignSecrets, err := findCosignSecrets(secretLocation)
	if err != nil {
		return fmt.Errorf("error finding cosign signing keys: %v", err)
	}

	if len(cosignSecrets) == 0 {
		s.Logger.Println("no keys found for cosign signing")
		return nil
	}

	if len(report.Image.Tags) == 0 {
		s.Logger.Println("no image found in report to sign")
		return nil
	}

	refImage := report.Image.Tags[0]

	for _, cosignSecret := range cosignSecrets {
		if err := s.sign(ctx, refImage, cosignSecret, secretLocation, annotations, cosignRepositories, cosignDockerMediaTypes); err != nil {
			return err
		}
	}

	return nil
}

func (s *ImageSigner) sign(ctx context.Context, refImage, cosignSecret, secretLocation string, annotations, cosignRepositories, cosignDockerMediaTypes map[string]interface{}) error {
	cosignKeyFile := fmt.Sprintf("%s/%s/cosign.key", secretLocation, cosignSecret)
	cosignPasswordFile := fmt.Sprintf("%s/%s/cosign.password", secretLocation, cosignSecret)

	ko := cli.KeyOpts{KeyRef: cosignKeyFile, PassFunc: func(bool) ([]byte, error) {
		content, err := ioutil.ReadFile(cosignPasswordFile)
		// When password file is not available, default empty password is used
		if err != nil {
			return []byte(""), nil
		}

		return content, nil
	}}

	if cosignRepository, ok := cosignRepositories[cosignSecret]; ok {
		if err := os.Setenv(cosignRepositoryEnv, fmt.Sprintf("%s", cosignRepository)); err != nil {
			return fmt.Errorf("failed setting %s env variable: %v", cosignRepositoryEnv, err)
		}
		defer os.Unsetenv(cosignRepositoryEnv)
	}

	if cosignDockerMediaType, ok := cosignDockerMediaTypes[cosignSecret]; ok {
		if err := os.Setenv(cosignDockerMediaTypesEnv, fmt.Sprintf("%s", cosignDockerMediaType)); err != nil {
			return fmt.Errorf("failed setting %s env variable: %v", cosignDockerMediaTypesEnv, err)
		}
		defer os.Unsetenv(cosignDockerMediaTypesEnv)
	}

	if err := s.signFunc(ctx, ko, annotations, refImage, "", true, "", false, false); err != nil {
		return fmt.Errorf("unable to sign image with %s: %v\n", cosignKeyFile, err)
	}

	return nil
}

func findCosignSecrets(secretLocation string) ([]string, error) {
	var result []string

	files, err := ioutil.ReadDir(secretLocation)
	if err != nil {
		return nil, err
	}

	for _, path := range files {
		if path.IsDir() {
			result = append(result, path.Name())
		}
	}

	return result, nil
}
