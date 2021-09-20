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

var (
	secretLocation            = "/var/build-secrets/cosign"
	cosignRepositoryEnv       = "COSIGN_REPOSITORY"
	cosignDockerMediaTypesEnv = "COSIGN_DOCKER_MEDIA_TYPES"
)

func NewImageSigner(logger *log.Logger, signFunc SignFunc) *ImageSigner {
	return &ImageSigner{
		Logger:   logger,
		signFunc: signFunc,
	}
}

func (s *ImageSigner) Sign(ctx context.Context, report lifecycle.ExportReport, annotations map[string]interface{}, cosignRepositories map[string]interface{}, cosignDockerMediaTypes map[string]interface{}) error {
	cosignSecrets, err := findCosignSecrets()
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
				return fmt.Errorf("failed setting COSIGN_REPOSITORY env variable: %v", err)
			}
		}

		if cosignDockerMediaType, ok := cosignDockerMediaTypes[cosignSecret]; ok {
			if err := os.Setenv(cosignDockerMediaTypesEnv, fmt.Sprintf("%s", cosignDockerMediaType)); err != nil {
				return fmt.Errorf("failed setting COSIGN_DOCKER_MEDIA_TYPES env variable: %v", err)
			}
		}

		// Separate error catching because each function should be attempted
		errorString := ""
		if err := s.signFunc(ctx, ko, annotations, refImage, "", true, "", false, false); err != nil {
			errorString = fmt.Sprintf("unable to sign image with %s: %v\n", cosignKeyFile, err)
		}
		if err := os.Unsetenv(cosignRepositoryEnv); err != nil {
			errorString = fmt.Sprintf("%sfailed unsetting COSIGN_REPOSITORY variable: %v\n", errorString, err)
		}
		if err := os.Unsetenv(cosignDockerMediaTypesEnv); err != nil {
			errorString = fmt.Sprintf("%sfailed unsetting COSIGN_DOCKER_MEDIA_TYPES variable: %v\n", errorString, err)
		}

		if errorString != "" {
			return fmt.Errorf(errorString)
		}
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
