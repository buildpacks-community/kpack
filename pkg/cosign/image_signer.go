package cosign

import (
	"context"
	"fmt"
	"os"

	"github.com/buildpacks/lifecycle/platform/files"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
	cosignoptions "github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/secret"
)

const (
	CosignRepositoryEnv       = "COSIGN_REPOSITORY"
	CosignDockerMediaTypesEnv = "COSIGN_DOCKER_MEDIA_TYPES"
)

type SignFunc func(*cosignoptions.RootOptions, cosignoptions.KeyOpts, cosignoptions.SignOptions, []string) error

type FetchSignatureFunc func(name.Reference, ...cosignremote.Option) (name.Tag, error)

type BuilderSigner interface {
	SignBuilder(context.Context, string, []*corev1.Secret, authn.Keychain) ([]v1alpha2.CosignSignature, error)
}

type ImageSigner struct {
	signFunc           SignFunc
	fetchSignatureFunc FetchSignatureFunc
}

func NewImageSigner(signFunc SignFunc, fetchSignatureFunc FetchSignatureFunc) *ImageSigner {
	return &ImageSigner{
		signFunc:           signFunc,
		fetchSignatureFunc: fetchSignatureFunc,
	}
}

func (s *ImageSigner) Sign(ro *cosignoptions.RootOptions, report files.Report, secretLocation string, annotations, cosignRepositories, cosignDockerMediaTypes map[string]interface{}) error {
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

func (s *ImageSigner) sign(ro *cosignoptions.RootOptions, refImage, digest, secretLocation, cosignSecret string, annotations, cosignRepositories, cosignDockerMediaTypes map[string]interface{}) error {
	cosignKeyFile := fmt.Sprintf("%s/%s/cosign.key", secretLocation, cosignSecret)
	cosignPasswordFile := fmt.Sprintf("%s/%s/cosign.password", secretLocation, cosignSecret)

	ko := cosignoptions.KeyOpts{KeyRef: cosignKeyFile, PassFunc: func(bool) ([]byte, error) {
		content, err := os.ReadFile(cosignPasswordFile)
		// When password file is not available, default empty password is used
		if err != nil {
			return []byte(""), nil
		}

		return content, nil
	}}

	if cosignRepository, ok := cosignRepositories[cosignSecret]; ok {
		if err := os.Setenv(CosignRepositoryEnv, fmt.Sprintf("%s", cosignRepository)); err != nil {
			return errors.Errorf("failed setting %s env variable: %v", CosignRepositoryEnv, err)
		}
		defer os.Unsetenv(CosignRepositoryEnv)
	}

	if cosignDockerMediaType, ok := cosignDockerMediaTypes[cosignSecret]; ok {
		if err := os.Setenv(CosignDockerMediaTypesEnv, fmt.Sprintf("%s", cosignDockerMediaType)); err != nil {
			return errors.Errorf("failed setting COSIGN_DOCKER_MEDIA_TYPES env variable: %v", err)
		}
		defer os.Unsetenv(CosignDockerMediaTypesEnv)
	}

	var cosignAnnotations []string
	for key, value := range annotations {
		cosignAnnotations = append(cosignAnnotations, fmt.Sprintf("%s=%s", key, value))
	}

	signOptions := cosignoptions.SignOptions{
		Registry: cosignoptions.RegistryOptions{KubernetesKeychain: true},
		AnnotationOptions: cosignoptions.AnnotationOptions{
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

func (s *ImageSigner) SignBuilder(
	ctx context.Context,
	imageReference string,
	serviceAccountSecrets []*corev1.Secret,
	builderKeychain authn.Keychain,
) ([]v1alpha2.CosignSignature, error) {
	signaturePaths := make([]v1alpha2.CosignSignature, 0)
	cosignSecrets := secret.FilterCosignSigningSecrets(serviceAccountSecrets)

	for _, cosignSecret := range cosignSecrets {
		keyRef := fmt.Sprintf("k8s://%s/%s", cosignSecret.Namespace, cosignSecret.Name)
		keyOpts := cosignoptions.KeyOpts{
			KeyRef: keyRef,
			PassFunc: func(bool) ([]byte, error) {
				if password, ok := cosignSecret.Data[secret.CosignSecretPassword]; ok {
					return password, nil
				}

				return []byte(""), nil
			},
		}

		if cosignRepository, ok := cosignSecret.Annotations[secret.CosignRepositoryAnnotation]; ok {
			if err := os.Setenv(CosignRepositoryEnv, cosignRepository); err != nil {
				return nil, fmt.Errorf("failed setting %s env variable: %w", CosignRepositoryEnv, err)
			}
		}

		if cosignDockerMediaType, ok := cosignSecret.Annotations[secret.CosignDockerMediaTypesAnnotation]; ok {
			if err := os.Setenv(CosignDockerMediaTypesEnv, cosignDockerMediaType); err != nil {
				return nil, fmt.Errorf("failed setting %s env variable: %w", CosignDockerMediaTypesEnv, err)
			}
		}

		registryOptions := cosignoptions.RegistryOptions{KubernetesKeychain: true, Keychain: builderKeychain}

		signOptions := cosignoptions.SignOptions{
			Registry:          registryOptions,
			AnnotationOptions: cosignoptions.AnnotationOptions{},
			Upload:            true,
			Recursive:         false,
			TlogUpload:        false,
		}

		rootOptions := cosignoptions.RootOptions{Timeout: cosignoptions.DefaultTimeout}

		if err := s.signFunc(
			&rootOptions,
			keyOpts,
			signOptions,
			[]string{imageReference}); err != nil {
			return nil, fmt.Errorf("unable to sign image with specified key from secret %s in namespace %s: %w", cosignSecret.Name, cosignSecret.Namespace, err)
		}

		reference, err := name.ParseReference(imageReference)
		if err != nil {
			return nil, fmt.Errorf("failed to parse reference: %w", err)
		}

		registryOpts, err := registryOptions.ClientOpts(ctx)
		if err != nil {
			return nil, err
		}

		signatureTag, err := s.fetchSignatureFunc(reference, registryOpts...)
		if err != nil {
			return nil, err
		}

		image, err := remote.Image(signatureTag, remote.WithAuthFromKeychain(builderKeychain))
		if err != nil {
			return nil, err
		}

		digest, err := image.Digest()
		if err != nil {
			return nil, err
		}

		signaturePaths = append(
			signaturePaths,
			v1alpha2.CosignSignature{
				SigningSecret: keyRef,
				TargetDigest:  signatureTag.Digest(digest.String()).String(),
			},
		)

		if _, found := os.LookupEnv(CosignDockerMediaTypesEnv); found {
			err = os.Unsetenv(CosignDockerMediaTypesEnv)
			if err != nil {
				return nil, fmt.Errorf("failed to cleanup environment variable %s: %w", CosignDockerMediaTypesEnv, err)
			}
		}

		if _, found := os.LookupEnv(CosignRepositoryEnv); found {
			err = os.Unsetenv(CosignRepositoryEnv)
			if err != nil {
				return nil, fmt.Errorf("failed to cleanup environment variable %s: %w", CosignRepositoryEnv, err)
			}
		}
	}

	return signaturePaths, nil
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
