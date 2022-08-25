package cosign

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/buildpacks/lifecycle/platform"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/cmd/cosign/cli/options"
)

type SignFunc func(
	ro *options.RootOptions, ko options.KeyOpts, regOpts options.RegistryOptions, annotations map[string]interface{},
	imgs []string, certPath string, certChainPath string, upload bool, outputSignature, outputCertificate string,
	payloadPath string, force bool, recursive bool, attachment string, noTlogUpload bool,
) error

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

func (s *ImageSigner) Sign(ro *options.RootOptions, report platform.ExportReport, secretLocation string, annotations, cosignRepositories, cosignDockerMediaTypes map[string]interface{}) error {
	cosignSecrets, err := findCosignSecrets(secretLocation)
	if err != nil {
		return errors.Errorf("no keys found for cosign signing: %v\n", err)
	}

	if len(cosignSecrets) == 0 {
		return errors.New("no keys found for cosign signing")
	}

	if len(report.Image.Tags) == 0 {
		return errors.New("no image found in report to sign")
	}

	refImage := report.Image.Tags[0]

	for _, cosignSecret := range cosignSecrets {
		if err := s.sign(ro, refImage, secretLocation, cosignSecret, annotations, cosignRepositories, cosignDockerMediaTypes); err != nil {
			return err
		}
	}

	return nil
}

func (s *ImageSigner) sign(ro *options.RootOptions, refImage, secretLocation, cosignSecret string, annotations, cosignRepositories, cosignDockerMediaTypes map[string]interface{}) error {
	cosignKeyFile := fmt.Sprintf("%s/%s/cosign.key", secretLocation, cosignSecret)
	cosignPasswordFile := fmt.Sprintf("%s/%s/cosign.password", secretLocation, cosignSecret)

	ko := options.KeyOpts{KeyRef: cosignKeyFile, PassFunc: func(bool) ([]byte, error) {
		content, err := ioutil.ReadFile(cosignPasswordFile)
		// When password file is not available, default empty password is used
		if err != nil {
			return []byte(""), nil
		}

		return content, nil
	}}

	if cosignRepository, ok := cosignRepositories[cosignSecret]; ok {
		if err := os.Setenv(cosignRepositoryEnv, fmt.Sprintf("%s", cosignRepository)); err != nil {
			return errors.Errorf("failed setting %s env variable: %v", cosignRepositoryEnv, err)
		}
		defer os.Unsetenv(cosignRepositoryEnv)
	}

	if cosignDockerMediaType, ok := cosignDockerMediaTypes[cosignSecret]; ok {
		if err := os.Setenv(cosignDockerMediaTypesEnv, fmt.Sprintf("%s", cosignDockerMediaType)); err != nil {
			return errors.Errorf("failed setting COSIGN_DOCKER_MEDIA_TYPES env variable: %v", err)
		}
		defer os.Unsetenv(cosignDockerMediaTypesEnv)
	}
	if err := s.signFunc(
		ro,
		ko,
		options.RegistryOptions{KubernetesKeychain: true},
		annotations,
		[]string{refImage},
		"",
		"",
		true,
		"",
		"",
		"",
		false,
		false,
		"",
		true); err != nil {
		return errors.Errorf("unable to sign image with %s: %v", cosignKeyFile, err)
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
