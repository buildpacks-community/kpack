package cosign

import (
	"fmt"
	"log"
	"os"

	"github.com/buildpacks/lifecycle/platform"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
)

type SignFunc func(
	ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
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
	digest := ""
	if report.Image.Digest != "" {
		digest = "@" + report.Image.Digest
	}

	for _, cosignSecret := range cosignSecrets {
		if err := s.sign(ro, refImage, digest, secretLocation, cosignSecret, annotations, cosignRepositories, cosignDockerMediaTypes); err != nil {
			return err
		}
	}

	return nil
}

func (s *ImageSigner) sign(ro *options.RootOptions, refImage, digest, secretLocation, cosignSecret string, annotations, cosignRepositories, cosignDockerMediaTypes map[string]interface{}) error {
	cosignKeyFile := fmt.Sprintf("%s/%s/cosign.key", secretLocation, cosignSecret)
	cosignPasswordFile := fmt.Sprintf("%s/%s/cosign.password", secretLocation, cosignSecret)

	ko := options.KeyOpts{KeyRef: cosignKeyFile, PassFunc: func(bool) ([]byte, error) {
		content, err := os.ReadFile(cosignPasswordFile)
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

	var cosignAnnotations []string
	for key, value := range annotations {
		cosignAnnotations = append(cosignAnnotations, fmt.Sprintf("%s=%s", key, value))
	}

	signOptions := options.SignOptions{
		Registry: options.RegistryOptions{KubernetesKeychain: true},
		AnnotationOptions: options.AnnotationOptions{
			Annotations: cosignAnnotations,
		},
		Upload:     true,
		Recursive:  false,
		TlogUpload: false,
	}

	if err := s.signFunc(
		ro,
		ko,
		signOptions,
		[]string{refImage + digest}); err != nil {
		return errors.Errorf("unable to sign image with %s: %v", cosignKeyFile, err)
	}

	return nil
}

func findCosignSecrets(secretLocation string) ([]string, error) {
	var result []string

	files, err := os.ReadDir(secretLocation)
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
