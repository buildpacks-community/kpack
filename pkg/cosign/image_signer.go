package cosign

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/buildpacks/lifecycle/platform"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/cmd/cosign/cli/sign"
	v1 "k8s.io/api/core/v1"
)

type SignFunc func(
	ro *options.RootOptions, ko options.KeyOpts, registryOptions options.RegistryOptions, annotations map[string]interface{}, imageRef []string,
	certPath string, certChainPath string, upload bool, outputSignature, outputCertificate string,
	payloadPath string, force, recursive bool, attachment string,
) error

type ImageSigner struct {
	Logger   *log.Logger
	signFunc SignFunc
}

const (
	cosignRepositoryEnv       = "COSIGN_REPOSITORY"
	cosignDockerMediaTypesEnv = "COSIGN_DOCKER_MEDIA_TYPES"

	COSIGNSecretDataCosignKey              = "cosign.key"
	COSIGNSecretDataCosignPassword         = "cosign.password"
	COSIGNSecretDataCosignPublicKey        = "cosign.pub"
	COSIGNDockerMediaTypesAnnotationPrefix = "kpack.io/cosign.docker-media-types"
	COSIGNRespositoryAnnotationPrefix      = "kpack.io/cosign.repository"
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
		""); err != nil {
		return errors.Errorf("unable to sign image with %s: %v", cosignKeyFile, err)
	}

	return nil
}

func (s *ImageSigner) SignBuilder(ctx context.Context, refImage string, cosignSecrets []v1.Secret) ([]string, error) {
	for _, secret := range cosignSecrets {
		ko := sign.KeyOpts{
			KeyRef: fmt.Sprintf("k8s://%v/%v", secret.Namespace, secret.Name),
			PassFunc: func(bool) ([]byte, error) {
				if password, ok := secret.Data[COSIGNSecretDataCosignPassword]; ok {
					return password, nil
				}

				return []byte(""), nil
			},
		}

		if cosignRepository, ok := secret.Annotations[COSIGNRespositoryAnnotationPrefix]; ok {
			if err := os.Setenv(cosignRepositoryEnv, fmt.Sprintf("%s", cosignRepository)); err != nil {
				return []string{}, errors.Errorf("failed setting %s env variable: %v", cosignRepositoryEnv, err)
			}
			defer os.Unsetenv(cosignRepositoryEnv)
		}

		if cosignDockerMediaType, ok := secret.Annotations[COSIGNDockerMediaTypesAnnotationPrefix]; ok {
			if err := os.Setenv(cosignDockerMediaTypesEnv, fmt.Sprintf("%s", cosignDockerMediaType)); err != nil {
				return []string{}, errors.Errorf("failed setting COSIGN_DOCKER_MEDIA_TYPES env variable: %v", err)
			}
			defer os.Unsetenv(cosignDockerMediaTypesEnv)
		}

		if err := s.signFunc(
			ctx,
			ko,
			options.RegistryOptions{},
			make(map[string]interface{}, 0),
			[]string{refImage},
			"",
			true,
			"",
			"",
			"",
			false,
			false,
			""); err != nil {
			return []string{}, errors.Errorf("unable to sign image with specified key from secret %v in namespace %v: %v", secret.Name, secret.Namespace, err)
		}

		// TODO find signature path when successful
	}

	return []string{}, nil
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
