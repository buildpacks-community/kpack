package cosigner

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/buildpacks/lifecycle"
	"github.com/sigstore/cosign/cmd/cosign/cli"
)

type SignFunc func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef string, certPath string, upload bool, payloadPath string, force bool, recursive bool) error

type ImageSigner struct {
	Logger   *log.Logger
	signFunc SignFunc
}

const (
	cosignRepositoryEnv       = "COSIGN_REPOSITORY"
	cosignDockerMediaTypesEnv = "COSIGN_DOCKER_MEDIA_TYPES"
)

var (
	secretLocation = "/var/build-secrets/cosign"
)

func NewImageSigner(logger *log.Logger, signFunc SignFunc) *ImageSigner {
	return &ImageSigner{
		Logger:   logger,
		signFunc: signFunc,
	}
}

func (s *ImageSigner) Sign(report lifecycle.ExportReport, annotations map[string]interface{}, cosignRepositories map[string]interface{}, cosignDockerMediaTypes map[string]interface{}) error {
	if len(report.Image.Tags) < 1 {
		s.Logger.Println("no image tag to sign")
		return nil
	}

	cosignSecrets, err := findCosignSecrets()
	if err != nil {
		s.Logger.Printf("no keys found for cosign signing: %v\n", err)
		return nil
	}

	if len(cosignSecrets) == 0 {
		s.Logger.Println("no keys found for cosign signing")
		return nil
	}

	refImage := report.Image.Tags[0]

	ctx := context.Background()
	for _, cosignSecret := range cosignSecrets {
		cosignKeyFile := fmt.Sprintf("%s/%s/cosign.key", secretLocation, cosignSecret)
		cosignPasswordFile := fmt.Sprintf("%s/%s/cosign.password", secretLocation, cosignSecret)

		ko := cli.KeyOpts{KeyRef: cosignKeyFile, PassFunc: func(bool) ([]byte, error) {
			content, err := ioutil.ReadFile(cosignPasswordFile)
			if err != nil {
				return []byte(""), nil
			}

			return content, nil
		}}

		if cosignRepository, ok := cosignRepositories[cosignSecret]; ok {
			os.Setenv(cosignRepositoryEnv, fmt.Sprintf("%s", cosignRepository))
		}

		if cosignDockerMediaType, ok := cosignDockerMediaTypes[cosignSecret]; ok {
			os.Setenv(cosignDockerMediaTypesEnv, fmt.Sprintf("%s", cosignDockerMediaType))
		}

		if err := s.signFunc(ctx, ko, annotations, refImage, "", true, "", false, false); err != nil {
			os.Unsetenv(cosignRepositoryEnv)
			os.Unsetenv(cosignDockerMediaTypesEnv)

			return fmt.Errorf("unable to sign image with %s: %v", cosignKeyFile, err)
		}

		os.Unsetenv(cosignRepositoryEnv)
		os.Unsetenv(cosignDockerMediaTypesEnv)
	}

	return nil
}

func findCosignSecrets() ([]string, error) {
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
