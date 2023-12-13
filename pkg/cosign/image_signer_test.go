package cosign

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle/platform/files"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/download"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
	sigstoreCosign "github.com/sigstore/cosign/v2/pkg/cosign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cosigntesting "github.com/pivotal/kpack/pkg/cosign/testing"
	registry2 "github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
	"github.com/pivotal/kpack/pkg/secret"
)

var fetchSignatureFunc = func(_ name.Reference, options ...ociremote.Option) (name.Tag, error) {
	tag, _ := name.NewTag("test", nil)
	return tag, nil
}

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Cosign Image Signer Main", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		ro                = &options.RootOptions{Timeout: options.DefaultTimeout}
		report            files.Report
		reader            *os.File
		writer            *os.File
		imageDigest       string
		hash              v1.Hash
		stopRegistry      func()
		imageCleanup      func()
		repo              string
		expectedImageName string
	)

	it.Before(func() {
		_, reader, writer = mockLogger(t)
		repo, stopRegistry = fakeRegistry(t)

		expectedImageName = path.Join(repo, "test-cosign-image")

		hash, imageCleanup = pushRandomImage(t, expectedImageName)
		imageDigest = hash.String()
	})

	it.After(func() {
		stopRegistry()
		imageCleanup()
		resetLogger(reader, writer)
	})

	when("#Sign", func() {
		var (
			secretKey1     string
			publicKey1     string
			publicKey2     string
			passwordFile1  string
			passwordFile2  string
			secretLocation string
		)

		when("signing occurs", func() {
			it.Before(func() {
				// Override secretLocation for test
				secretLocation = createCosignKeyFiles(t)

				secretKey1 = path.Join(secretLocation, "secret-name-1", secret.CosignSecretPrivateKey)
				publicKey1 = path.Join(secretLocation, "secret-name-1", secret.CosignSecretPublicKey)
				publicKey2 = path.Join(secretLocation, "secret-name-2", secret.CosignSecretPublicKey)
				passwordFile1 = path.Join(secretLocation, "secret-name-1", secret.CosignSecretPassword)
				passwordFile2 = path.Join(secretLocation, "secret-name-2", secret.CosignSecretPassword)

				report = createReportToml(t, expectedImageName, imageDigest)

				os.Unsetenv(CosignRepositoryEnv)
				os.Unsetenv(CosignDockerMediaTypesEnv)
			})

			it("signs images", func() {
				cliSignCmdCallCount := 0
				password1Count := 0
				password2Count := 0

				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					t.Helper()
					expectedImageNameWithDigest := expectedImageName + "@" + imageDigest
					assert.Equal(t, []string{expectedImageNameWithDigest}, imgs)

					// Test key location
					assert.Contains(t, ko.KeyRef, "cosign.key")
					assert.Contains(t, ko.KeyRef, secretLocation)

					password, err := ko.PassFunc(true)
					assert.Nil(t, err)

					var passwordFileContent []byte
					if secretKey1 == ko.KeyRef {
						passwordFileContent, _ = os.ReadFile(passwordFile1)
						password1Count++
						assert.Equal(t, []byte(""), passwordFileContent)
					} else {
						passwordFileContent, _ = os.ReadFile(passwordFile2)
						password2Count++
						assert.NotEqual(t, []byte(""), passwordFileContent)
					}
					assert.Equal(t, passwordFileContent, password)

					assert.Empty(t, signOpts.AnnotationOptions.Annotations)
					cliSignCmdCallCount++
					return sign.SignCmd(
						ro,
						ko,
						signOpts,
						imgs)
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 2, cliSignCmdCallCount)
				assert.Equal(t, 1, password1Count)
				assert.Equal(t, 1, password2Count)

				err = cosigntesting.Verify(t, publicKey1, expectedImageName, nil)
				assert.Nil(t, err)

				err = cosigntesting.Verify(t, publicKey2, expectedImageName, nil)
				assert.Nil(t, err)

				err = download.SignatureCmd(context.Background(), options.RegistryOptions{}, expectedImageName)
				assert.Nil(t, err)
			})

			it("signs with annotations", func() {
				expectedAnnotation := map[string]interface{}{
					"annotationKey1": "value1",
				}

				unexpectedAnnotation := map[string]interface{}{
					"annotationKey1": "value2",
				}

				cliSignCmdCallCount := 0
				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					t.Helper()
					expectedImageNameWithDigest := expectedImageName + "@" + imageDigest
					assert.Equal(t, []string{expectedImageNameWithDigest}, imgs)
					assert.Contains(t, ko.KeyRef, "cosign.key")
					assert.Contains(t, ko.KeyRef, secretLocation)
					assert.Equal(t, []string{"annotationKey1=value1"}, signOpts.AnnotationOptions.Annotations)
					cliSignCmdCallCount++
					return sign.SignCmd(
						ro,
						ko,
						signOpts,
						imgs,
					)
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, expectedAnnotation, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 2, cliSignCmdCallCount)

				// Should error when validating annotations that dont exist
				err = cosigntesting.Verify(t, publicKey1, expectedImageName, unexpectedAnnotation)
				assert.Error(t, err)
				err = cosigntesting.Verify(t, publicKey2, expectedImageName, unexpectedAnnotation)
				assert.Error(t, err)

				// Should not error when validating annotations that exist
				err = cosigntesting.Verify(t, publicKey1, expectedImageName, expectedAnnotation)
				assert.Nil(t, err)
				err = cosigntesting.Verify(t, publicKey2, expectedImageName, expectedAnnotation)
				assert.Nil(t, err)

				// Should not error when not validating annotations
				err = cosigntesting.Verify(t, publicKey1, expectedImageName, nil)
				assert.Nil(t, err)
				err = cosigntesting.Verify(t, publicKey2, expectedImageName, nil)
				assert.Nil(t, err)

				err = download.SignatureCmd(context.Background(), options.RegistryOptions{}, expectedImageName)
				assert.Nil(t, err)
			})
			it("errors early when signing fails", func() {
				cliSignCmdCallCount := 0

				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					cliSignCmdCallCount++
					return sign.SignCmd(
						ro,
						ko,
						signOpts,
						imgs,
					)
				}

				emptyKey := filepath.Join(secretLocation, "secret-name-0")
				os.Mkdir(filepath.Join(secretLocation, "secret-name-0"), 0700)
				expectedErrorMessage := fmt.Sprintf("unable to sign image with %s/cosign.key: getting signer: reading key: open %s/cosign.key: no such file or directory", emptyKey, emptyKey)

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, nil, nil)
				assert.Error(t, err)
				assert.Equal(t, expectedErrorMessage, err.Error())
				assert.Equal(t, 1, cliSignCmdCallCount)
			})

			it("errors when signing fails", func() {
				cliSignCmdCallCount := 0

				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					cliSignCmdCallCount++
					return sign.SignCmd(
						ro,
						ko,
						signOpts,
						imgs,
					)
				}

				emptyKey := filepath.Join(secretLocation, "secret-name-3")
				os.Mkdir(filepath.Join(secretLocation, "secret-name-3"), 0700)
				expectedErrorMessage := fmt.Sprintf("unable to sign image with %s/cosign.key: getting signer: reading key: open %s/cosign.key: no such file or directory", emptyKey, emptyKey)

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, nil, nil)
				assert.Error(t, err)
				assert.Equal(t, expectedErrorMessage, err.Error())
				assert.Equal(t, 3, cliSignCmdCallCount)
			})

			it("sets COSIGN_REPOSITORY environment variable", func() {
				altRepo, altStopRegistry := fakeRegistry(t)
				defer altStopRegistry()
				altImageName := path.Join(altRepo, "test-cosign-image-alt")

				cliSignCmdCallCount := 0

				assert.Empty(t, len(os.Getenv(CosignRepositoryEnv)))
				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					t.Helper()
					if strings.Contains(ko.KeyRef, "secret-name-2") {
						assert.Equal(t, altImageName, os.Getenv(CosignRepositoryEnv))
					} else {
						assertUnset(t, CosignRepositoryEnv)
					}

					cliSignCmdCallCount++
					return sign.SignCmd(
						ro,
						ko,
						signOpts,
						imgs,
					)
				}

				cosignRepositories := map[string]interface{}{
					"secret-name-2": altImageName,
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, cosignRepositories, nil)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, CosignRepositoryEnv)

				err = cosigntesting.Verify(t, publicKey1, expectedImageName, nil)
				assert.Nil(t, err)
				err = cosigntesting.Verify(t, publicKey2, expectedImageName, nil)
				assert.Error(t, err)
				err = download.SignatureCmd(context.Background(), options.RegistryOptions{}, expectedImageName)
				assert.Nil(t, err)

				// Required to set COSIGN_REPOSITORY env variable to validate signature
				// on a registry that does not contain the image
				os.Setenv(CosignRepositoryEnv, altImageName)
				defer os.Unsetenv(CosignRepositoryEnv)
				err = cosigntesting.Verify(t, publicKey1, expectedImageName, nil)
				assert.Error(t, err)
				err = cosigntesting.Verify(t, publicKey2, expectedImageName, nil)
				assert.Nil(t, err)
			})

			it("sets COSIGN_DOCKER_MEDIA_TYPES environment variable", func() {
				cliSignCmdCallCount := 0

				assertUnset(t, CosignDockerMediaTypesEnv)
				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					t.Helper()
					if strings.Contains(ko.KeyRef, "secret-name-1") {
						assert.Equal(t, "1", os.Getenv(CosignDockerMediaTypesEnv))
					} else {
						assertUnset(t, CosignDockerMediaTypesEnv)
					}

					cliSignCmdCallCount++
					return nil
				}

				cosignDockerMediaTypes := map[string]interface{}{
					"secret-name-1": "1",
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, nil, cosignDockerMediaTypes)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, CosignDockerMediaTypesEnv)
			})

			it("sets both COSIGN_REPOSITORY and COSIGN_DOCKER_MEDIA_TYPES environment variable", func() {
				cliSignCmdCallCount := 0

				assertUnset(t, CosignDockerMediaTypesEnv)
				assertUnset(t, CosignRepositoryEnv)
				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					t.Helper()
					assert.Equal(t, "1", os.Getenv(CosignDockerMediaTypesEnv))
					assert.Equal(t, "registry.example.com/fakeproject", os.Getenv(CosignRepositoryEnv))
					cliSignCmdCallCount++
					return nil
				}

				cosignRepositories := map[string]interface{}{
					"secret-name-1": "registry.example.com/fakeproject",
					"secret-name-2": "registry.example.com/fakeproject",
				}

				cosignDockerMediaTypes := map[string]interface{}{
					"secret-name-1": "1",
					"secret-name-2": "1",
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, cosignRepositories, cosignDockerMediaTypes)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, CosignDockerMediaTypesEnv)
				assertUnset(t, CosignRepositoryEnv)
			})
		})

		when("signing returns error", func() {
			it("has no cosign secrets", func() {
				secretLocation = t.TempDir()
				report = createReportToml(t, expectedImageName, imageDigest)

				cliSignCmdCallCount := 0
				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					t.Helper()
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, nil, nil)
				require.Error(t, err, "no keys found for cosign signing")
				assert.Equal(t, 0, cliSignCmdCallCount)
			})

			it("has invalid directory", func() {
				secretLocation = "/fake/location/that/doesnt/exist"
				report = createReportToml(t, expectedImageName, imageDigest)

				cliSignCmdCallCount := 0
				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, nil, nil)
				require.Error(t, err, "no keys found for cosign signing: open /fake/location/that/doesnt/exist: no such file or directory")
				assert.Equal(t, 0, cliSignCmdCallCount)
			})

			it("has no image.Tags in report", func() {
				secretLocation = createCosignKeyFiles(t)
				report = createEmptyReportToml(t)

				cliSignCmdCallCount := 0
				cliSignCmd := func(
					ro *options.RootOptions, ko options.KeyOpts, signOpts options.SignOptions, imgs []string,
				) error {
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(cliSignCmd, fetchSignatureFunc)
				err := signer.Sign(ro, report, secretLocation, nil, nil, nil)
				require.Error(t, err, "no image found in report to sign")
				assert.Equal(t, 0, cliSignCmdCallCount)
			})
		})
	})

	when("#Cosign.SignCmd", func() {
		it("signs an image", func() {
			secretLocation := t.TempDir()

			repo, stop := fakeRegistry(t)
			defer stop()

			imgName := path.Join(repo, "cosign-e2e")

			_, cleanup := pushRandomImage(t, imgName)
			defer cleanup()

			password := ""
			keypair(t, secretLocation, "secret-name-1", password)
			privKeyPath := path.Join(secretLocation, "secret-name-1", secret.CosignSecretPrivateKey)
			pubKeyPath := path.Join(secretLocation, "secret-name-1", secret.CosignSecretPublicKey)

			ctx := context.Background()
			// Verify+download should fail at first
			err := cosigntesting.Verify(t, pubKeyPath, imgName, nil)
			assert.Error(t, err)
			err = download.SignatureCmd(ctx, options.RegistryOptions{}, imgName)
			assert.Error(t, err)

			// Sign
			passFunc := func(_ bool) ([]byte, error) {
				return []byte(password), nil
			}
			ko := options.KeyOpts{KeyRef: privKeyPath, PassFunc: passFunc}
			signOptions := options.SignOptions{
				Registry:   options.RegistryOptions{},
				Upload:     true,
				Recursive:  false,
				TlogUpload: false,
			}
			err = sign.SignCmd(
				ro,
				ko,
				signOptions,
				[]string{imgName},
			)
			assert.Nil(t, err)

			// Verify+download should pass
			err = cosigntesting.Verify(t, pubKeyPath, imgName, nil)
			assert.Nil(t, err)
			err = download.SignatureCmd(ctx, options.RegistryOptions{}, imgName)
			assert.Nil(t, err)
		})
	})

	when("#SignBuilder", func() {
		const (
			cosignSecretName         = "cosign-creds"
			testNamespaceName        = "test-namespace"
			cosignServiceAccountName = "cosign-sa"
		)

		it("resolves the digest of a signature correctly", func() {
			var (
				signCallCount           = 0
				fetchSignatureCallCount = 0
			)

			fakeImageSignatureTag := fmt.Sprintf("%s:%s", expectedImageName, "test.sig")
			digest, cleanup := pushRandomImage(t, fakeImageSignatureTag)
			defer cleanup()

			fakeImageSigner := &ImageSigner{
				signFunc: func(rootOptions *options.RootOptions, opts options.KeyOpts, signOptions options.SignOptions, i []string) error {
					t.Helper()

					signCallCount++
					return nil
				},
				fetchSignatureFunc: func(reference name.Reference, option ...ociremote.Option) (name.Tag, error) {
					t.Helper()

					fetchSignatureCallCount++
					return name.NewTag(fakeImageSignatureTag)
				},
			}

			fakeSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespaceName, "", nil)
			cosignCreds := []*corev1.Secret{&fakeSecret}
			cosignSA := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cosignServiceAccountName,
					Namespace: testNamespaceName,
				},
				Secrets: []corev1.ObjectReference{
					{
						Name: fakeSecret.Name,
					},
				},
			}

			secretRef := registry2.SecretRef{
				ServiceAccount: cosignSA.Name,
				Namespace:      cosignSA.Namespace,
			}

			keychainFactory := &registryfakes.FakeKeychainFactory{}
			fakeKeychain := &registryfakes.FakeKeychain{}
			keychainFactory.AddKeychainForSecretRef(t, secretRef, fakeKeychain)

			signaturePaths, err := fakeImageSigner.SignBuilder(context.Background(), expectedImageName, cosignCreds, fakeKeychain)
			require.NoError(t, err)
			require.NotEmpty(t, signaturePaths)
			require.NotNil(t, signaturePaths[0])

			assert.Contains(t, signaturePaths[0].TargetDigest, digest.String())
			assert.Contains(t, signaturePaths[0].SigningSecret, fakeSecret.Namespace)
			assert.Contains(t, signaturePaths[0].SigningSecret, fakeSecret.Name)

			require.Equal(t, 1, signCallCount)
			require.Equal(t, 1, fetchSignatureCallCount)
		})

		it("sets environment variables when needed", func() {
			var (
				signCallCount           = 0
				fetchSignatureCallCount = 0
				signaturesPath          = path.Join(repo, "signatures")
				dockerMediaTypesValue   = "1"
			)

			fakeImageSignatureTag := fmt.Sprintf("%s:%s", signaturesPath, "test.sig")
			digest, cleanup := pushRandomImage(t, fakeImageSignatureTag)
			defer cleanup()

			fakeImageSigner := &ImageSigner{
				signFunc: func(rootOptions *options.RootOptions, opts options.KeyOpts, signOptions options.SignOptions, i []string) error {
					t.Helper()

					value, found := os.LookupEnv(CosignRepositoryEnv)
					require.True(t, found)
					require.NotNil(t, value)
					assert.Equal(t, signaturesPath, value)

					value, found = os.LookupEnv(CosignDockerMediaTypesEnv)
					require.True(t, found)
					require.NotNil(t, value)
					assert.Equal(t, dockerMediaTypesValue, value)

					signCallCount++
					return nil
				},
				fetchSignatureFunc: func(reference name.Reference, option ...ociremote.Option) (name.Tag, error) {
					t.Helper()

					fetchSignatureCallCount++
					return name.NewTag(fakeImageSignatureTag)
				},
			}

			annotations := map[string]string{
				"kpack.io/cosign.repository":         signaturesPath,
				"kpack.io/cosign.docker-media-types": dockerMediaTypesValue,
			}

			fakeSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespaceName, "", annotations)
			cosignCreds := []*corev1.Secret{&fakeSecret}
			cosignSA := corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cosignServiceAccountName,
					Namespace: testNamespaceName,
				},
				Secrets: []corev1.ObjectReference{
					{
						Name: fakeSecret.Name,
					},
				},
			}

			secretRef := registry2.SecretRef{
				ServiceAccount: cosignSA.Name,
				Namespace:      cosignSA.Namespace,
			}

			keychainFactory := &registryfakes.FakeKeychainFactory{}
			fakeKeychain := &registryfakes.FakeKeychain{}
			keychainFactory.AddKeychainForSecretRef(t, secretRef, fakeKeychain)

			signaturePaths, err := fakeImageSigner.SignBuilder(context.Background(), expectedImageName, cosignCreds, fakeKeychain)
			require.NoError(t, err)
			require.NotEmpty(t, signaturePaths)
			require.NotNil(t, signaturePaths[0])

			assert.Contains(t, signaturePaths[0].TargetDigest, digest.String())
			assert.Contains(t, signaturePaths[0].SigningSecret, fakeSecret.Namespace)
			assert.Contains(t, signaturePaths[0].SigningSecret, fakeSecret.Name)

			require.Equal(t, 1, signCallCount)
			require.Equal(t, 1, fetchSignatureCallCount)
		})
	})
}

func mockLogger(t *testing.T) (*bufio.Scanner, *os.File, *os.File) {
	reader, writer, err := os.Pipe()
	if err != nil {
		assert.Fail(t, "couldn't get os Pipe: %v", err)
	}
	log.SetOutput(writer)

	return bufio.NewScanner(reader), reader, writer
}

func resetLogger(reader *os.File, writer *os.File) {
	err := reader.Close()
	if err != nil {
		fmt.Println("error closing reader was ", err)
	}
	if err = writer.Close(); err != nil {
		fmt.Println("error closing writer was ", err)
	}
	log.SetOutput(os.Stderr)
}

func createCosignKeyFiles(t *testing.T) string {
	dirPath := t.TempDir()

	keypair(t, dirPath, "secret-name-1", "")
	keypair(t, dirPath, "secret-name-2", "testPassword")

	return dirPath
}

func createReportToml(t *testing.T, imageRef, imageDigest string) files.Report {
	var r files.Report
	_, err := toml.Decode(fmt.Sprintf(`[image]
	tags = ["%s"]`, imageRef), &r)
	_, err = toml.Decode(fmt.Sprintf(`[image]
	digest = "%s"`, imageDigest), &r)
	assert.Nil(t, err)
	return r
}

func createEmptyReportToml(t *testing.T) files.Report {
	var r files.Report
	_, err := toml.Decode(`[image]`, &r)
	assert.Nil(t, err)
	return r
}

func assertUnset(t *testing.T, envName string, msg ...string) {
	value, isSet := os.LookupEnv(envName)
	assert.False(t, isSet)
	assert.Equal(t, "", value)
}

func fakeRegistry(t *testing.T) (string, func()) {
	sinkLogger := log.New(io.Discard, "", 0)
	r := httptest.NewServer(registry.New(registry.Logger(sinkLogger)))
	u, err := url.Parse(r.URL)
	assert.Nil(t, err)

	return u.Host, r.Close
}

func pushRandomImage(t *testing.T, imageRef string) (v1.Hash, func()) {
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	require.NoError(t, err)

	img, err := random.Image(512, 5)
	require.NoError(t, err)

	regClientOpts := registryClientOpts(context.Background())

	err = remote.Write(ref, img, regClientOpts...)
	require.NoError(t, err)

	resp, err := remote.Get(ref, regClientOpts...)
	require.NoError(t, err)

	cleanup := func() {
		_ = remote.Delete(ref, regClientOpts...)
		ref, _ := ociremote.SignatureTag(ref, ociremote.WithRemoteOptions(regClientOpts...))
		_ = remote.Delete(ref, regClientOpts...)
	}

	return resp.Digest, cleanup
}

func registryClientOpts(ctx context.Context) []remote.Option {
	return []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	}
}

func keypair(t *testing.T, dirPath, secretName, password string) {
	t.Helper()

	passFunc := func(_ bool) ([]byte, error) {
		return []byte(password), nil
	}

	keys, err := sigstoreCosign.GenerateKeyPair(passFunc)
	assert.Nil(t, err)

	err = os.Mkdir(filepath.Join(dirPath, secretName), 0700)
	assert.Nil(t, err)

	privKeyPath := filepath.Join(dirPath, secretName, secret.CosignSecretPrivateKey)
	err = os.WriteFile(privKeyPath, keys.PrivateBytes, 0600)
	assert.Nil(t, err)

	pubKeyPath := filepath.Join(dirPath, secretName, secret.CosignSecretPublicKey)
	err = os.WriteFile(pubKeyPath, keys.PublicBytes, 0600)
	assert.Nil(t, err)

	passwordPath := filepath.Join(dirPath, secretName, secret.CosignSecretPassword)
	passwordBytes, _ := passFunc(true)
	err = os.WriteFile(passwordPath, passwordBytes, 0600)
	assert.Nil(t, err)
}
