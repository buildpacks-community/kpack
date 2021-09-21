package cosigner

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/cmd/cosign/cli"
	"github.com/sigstore/cosign/cmd/cosign/cli/download"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/stretchr/testify/assert"
)

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Cosign Image Signer Main", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		report            lifecycle.ExportReport
		reader            *os.File
		writer            *os.File
		scanner           *bufio.Scanner
		expectedImageName string
		stopRegistry      func()
		imageCleanup      func()
		repo              string
		testCtx           context.Context
		testCtxCancel     context.CancelFunc
	)

	it.Before(func() {
		testCtx, testCtxCancel = context.WithTimeout(context.Background(), time.Minute)
		scanner, reader, writer = mockLogger(t)
		repo, stopRegistry = reg(t)

		expectedImageName = path.Join(repo, "test-cosign-image")

		imageCleanup = pushRandomImage(t, expectedImageName)
	})

	it.After(func() {
		testCtxCancel()
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

				secretKey1 = path.Join(secretLocation, "secret-name-1", "cosign.key")
				publicKey1 = path.Join(secretLocation, "secret-name-1", "cosign.pub")
				publicKey2 = path.Join(secretLocation, "secret-name-2", "cosign.pub")
				passwordFile1 = path.Join(secretLocation, "secret-name-1", "cosign.password")
				passwordFile2 = path.Join(secretLocation, "secret-name-2", "cosign.password")

				report = createReportToml(t, expectedImageName)

				os.Unsetenv(cosignRepositoryEnv)
				os.Unsetenv(cosignDockerMediaTypesEnv)
			})

			it("signs images", func() {
				cliSignCmdCallCount := 0
				password1Count := 0
				password2Count := 0

				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, testCtx, ctx)
					assert.Equal(t, expectedImageName, imageRef)

					// Test key location
					assert.Contains(t, ko.KeyRef, "cosign.key")
					assert.Contains(t, ko.KeyRef, secretLocation)

					password, err := ko.PassFunc(true)
					assert.Nil(t, err)

					var passwordFileContent []byte
					if secretKey1 == ko.KeyRef {
						passwordFileContent, _ = ioutil.ReadFile(passwordFile1)
						password1Count++
						assert.Equal(t, []byte(""), passwordFileContent)
					} else {
						passwordFileContent, _ = ioutil.ReadFile(passwordFile2)
						password2Count++
						assert.NotEqual(t, []byte(""), passwordFileContent)
					}
					assert.Equal(t, passwordFileContent, password)

					assert.Nil(t, annotations)
					cliSignCmdCallCount++
					return cli.SignCmd(ctx, ko, annotations, imageRef, certPath, upload, payloadPath, force, recursive)
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 2, cliSignCmdCallCount)
				assert.Equal(t, 1, password1Count)
				assert.Equal(t, 1, password2Count)

				err = verify(publicKey1, expectedImageName, nil)
				assert.Nil(t, err)

				err = verify(publicKey2, expectedImageName, nil)
				assert.Nil(t, err)

				err = download.SignatureCmd(context.Background(), expectedImageName)
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
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, testCtx, ctx)
					assert.Equal(t, expectedImageName, imageRef)
					assert.Contains(t, ko.KeyRef, "cosign.key")
					assert.Contains(t, ko.KeyRef, secretLocation)
					assert.Equal(t, expectedAnnotation, annotations)
					cliSignCmdCallCount++
					return cli.SignCmd(ctx, ko, annotations, imageRef, certPath, upload, payloadPath, force, recursive)
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, expectedAnnotation, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 2, cliSignCmdCallCount)

				// Should error when validating annotations that dont exist
				err = verify(publicKey1, expectedImageName, unexpectedAnnotation)
				assert.Error(t, err)
				err = verify(publicKey2, expectedImageName, unexpectedAnnotation)
				assert.Error(t, err)

				// Should not error when validating annotations that exist
				err = verify(publicKey1, expectedImageName, expectedAnnotation)
				assert.Nil(t, err)
				err = verify(publicKey2, expectedImageName, expectedAnnotation)
				assert.Nil(t, err)

				// Should not error when not validating annotations
				err = verify(publicKey1, expectedImageName, nil)
				assert.Nil(t, err)
				err = verify(publicKey2, expectedImageName, nil)
				assert.Nil(t, err)

				err = download.SignatureCmd(context.Background(), expectedImageName)
				assert.Nil(t, err)
			})
			it("errors early when signing fails", func() {
				cliSignCmdCallCount := 0

				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					cliSignCmdCallCount++
					return cli.SignCmd(ctx, ko, annotations, imageRef, certPath, upload, payloadPath, force, recursive)
				}

				emptyKey := filepath.Join(secretLocation, "secret-name-0")
				os.Mkdir(filepath.Join(secretLocation, "secret-name-0"), 0700)
				expectedErrorMessage := fmt.Sprintf("unable to sign image with %s/cosign.key: getting signer: reading key: open %s/cosign.key: no such file or directory\n", emptyKey, emptyKey)

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, nil, nil)
				assert.Error(t, err)
				assert.Equal(t, expectedErrorMessage, err.Error())
				assert.Equal(t, 1, cliSignCmdCallCount)
			})

			it("errors when signing fails", func() {
				cliSignCmdCallCount := 0

				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					cliSignCmdCallCount++
					return cli.SignCmd(ctx, ko, annotations, imageRef, certPath, upload, payloadPath, force, recursive)
				}

				emptyKey := filepath.Join(secretLocation, "secret-name-3")
				os.Mkdir(filepath.Join(secretLocation, "secret-name-3"), 0700)
				expectedErrorMessage := fmt.Sprintf("unable to sign image with %s/cosign.key: getting signer: reading key: open %s/cosign.key: no such file or directory\n", emptyKey, emptyKey)

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, nil, nil)
				assert.Error(t, err)
				assert.Equal(t, expectedErrorMessage, err.Error())
				assert.Equal(t, 3, cliSignCmdCallCount)
			})

			it("sets COSIGN_REPOSITORY environment variable", func() {
				altRepo, altStopRegistry := reg(t)
				defer altStopRegistry()
				altImageName := path.Join(altRepo, "test-cosign-image-alt")

				cliSignCmdCallCount := 0

				assert.Empty(t, len(os.Getenv(cosignRepositoryEnv)))
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, testCtx, ctx)
					if strings.Contains(ko.KeyRef, "secret-name-2") {
						assert.Equal(t, altImageName, os.Getenv(cosignRepositoryEnv))
					} else {
						assertUnset(t, cosignRepositoryEnv)
					}

					cliSignCmdCallCount++
					return cli.SignCmd(ctx, ko, annotations, imageRef, certPath, upload, payloadPath, force, recursive)
				}

				cosignRepositories := map[string]interface{}{
					"secret-name-2": altImageName,
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, cosignRepositories, nil)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, cosignRepositoryEnv)

				err = verify(publicKey1, expectedImageName, nil)
				assert.Nil(t, err)
				err = verify(publicKey2, expectedImageName, nil)
				assert.Error(t, err)
				err = download.SignatureCmd(context.Background(), expectedImageName)
				assert.Nil(t, err)

				// Required to set COSIGN_REPOSITORY env variable to validate signature
				// on a registry that does not contain the image
				os.Setenv(cosignRepositoryEnv, altImageName)
				defer os.Unsetenv(cosignRepositoryEnv)
				err = verify(publicKey1, expectedImageName, nil)
				assert.Error(t, err)
				err = verify(publicKey2, expectedImageName, nil)
				assert.Nil(t, err)
			})

			it("sets COSIGN_DOCKER_MEDIA_TYPES environment variable", func() {
				cliSignCmdCallCount := 0

				assertUnset(t, cosignDockerMediaTypesEnv)
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, testCtx, ctx)
					if strings.Contains(ko.KeyRef, "secret-name-1") {
						assert.Equal(t, "1", os.Getenv(cosignDockerMediaTypesEnv))
					} else {
						assertUnset(t, cosignDockerMediaTypesEnv)
					}

					cliSignCmdCallCount++
					return nil
				}

				cosignDockerMediaTypes := map[string]interface{}{
					"secret-name-1": "1",
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, nil, cosignDockerMediaTypes)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, cosignDockerMediaTypesEnv)
			})

			it("sets both COSIGN_REPOSITORY and COSIGN_DOCKER_MEDIA_TYPES environment variable", func() {
				cliSignCmdCallCount := 0

				assertUnset(t, cosignDockerMediaTypesEnv)
				assertUnset(t, cosignRepositoryEnv)
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, testCtx, ctx)
					assert.Equal(t, "1", os.Getenv(cosignDockerMediaTypesEnv))
					assert.Equal(t, "registry.example.com/fakeproject", os.Getenv(cosignRepositoryEnv))
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

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, cosignRepositories, cosignDockerMediaTypes)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, cosignDockerMediaTypesEnv)
				assertUnset(t, cosignRepositoryEnv)
			})
		})

		when("signing is skipped because", func() {
			it("has no cosign secrets", func() {
				secretLocation = t.TempDir()
				report = createReportToml(t, expectedImageName)

				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, testCtx, ctx)
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 0, cliSignCmdCallCount)

				assert.Equal(t, "no keys found for cosign signing", logText(scanner))
			})

			it("has invalid directory", func() {
				secretLocation = "/fake/location/that/doesnt/exist"
				report = createReportToml(t, expectedImageName)

				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, nil, nil)
				assert.EqualError(t, err, "error finding cosign signing keys: open /fake/location/that/doesnt/exist: no such file or directory")

				assert.Equal(t, 0, cliSignCmdCallCount)
			})

			it("has no image.Tags in report", func() {
				secretLocation = createCosignKeyFiles(t)
				report = createEmptyReportToml(t)

				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(testCtx, report, secretLocation, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 0, cliSignCmdCallCount)

				assert.Equal(t, "no image found in report to sign", logText(scanner))
			})
		})
	})

	when("#Cosign.SignCmd", func() {
		it("signs an image", func() {
			secretLocation := t.TempDir()

			repo, stop := reg(t)
			defer stop()

			imgName := path.Join(repo, "cosign-e2e")

			cleanup := pushRandomImage(t, imgName)
			defer cleanup()

			password := ""
			keypair(t, secretLocation, "secret-name-1", password)
			privKeyPath := path.Join(secretLocation, "secret-name-1", "cosign.key")
			pubKeyPath := path.Join(secretLocation, "secret-name-1", "cosign.pub")

			ctx := context.Background()
			// Verify+download should fail at first
			err := verify(pubKeyPath, imgName, nil)
			assert.Error(t, err)
			err = download.SignatureCmd(ctx, imgName)
			assert.Error(t, err)

			// Sign
			passFunc := func(_ bool) ([]byte, error) {
				return []byte(password), nil
			}
			ko := cli.KeyOpts{KeyRef: privKeyPath, PassFunc: passFunc}
			err = cli.SignCmd(ctx, ko, nil, imgName, "", true, "", false, false)
			assert.Nil(t, err)

			// Verify+download should pass
			err = verify(pubKeyPath, imgName, nil)
			assert.Nil(t, err)
			err = download.SignatureCmd(ctx, imgName)
			assert.Nil(t, err)
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

func logText(scanner *bufio.Scanner) string {
	scanner.Scan()
	return scanner.Text()
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

func createReportToml(t *testing.T, imageRef string) lifecycle.ExportReport {
	var r lifecycle.ExportReport
	_, err := toml.Decode(fmt.Sprintf(`[image]
	tags = ["%s"]`, imageRef), &r)
	assert.Nil(t, err)
	return r
}

func createEmptyReportToml(t *testing.T) lifecycle.ExportReport {
	var r lifecycle.ExportReport
	_, err := toml.Decode(`[image]`, &r)
	assert.Nil(t, err)
	return r
}

func assertUnset(t *testing.T, envName string, msg ...string) {
	value, isSet := os.LookupEnv(envName)
	assert.False(t, isSet)
	assert.Equal(t, "", value)
}

func reg(t *testing.T) (string, func()) {
	r := httptest.NewServer(registry.New())
	u, err := url.Parse(r.URL)
	assert.Nil(t, err)

	return u.Host, r.Close
}

func pushRandomImage(t *testing.T, imageRef string) func() {
	ref, err := name.ParseReference(imageRef, name.WeakValidation)
	assert.Nil(t, err)

	img, err := random.Image(512, 5)
	assert.Nil(t, err)

	regClientOpts := registryClientOpts(context.Background())

	err = remote.Write(ref, img, regClientOpts...)
	assert.Nil(t, err)

	remoteImage, err := remote.Get(ref, regClientOpts...)
	assert.Nil(t, err)

	cleanup := func() {
		_ = remote.Delete(ref, regClientOpts...)
		ref := cosign.AttachedImageTag(ref.Context(), remoteImage.Descriptor.Digest, cosign.SignatureTagSuffix)
		_ = remote.Delete(ref, regClientOpts...)
	}

	return cleanup
}

func registryClientOpts(ctx context.Context) []remote.Option {
	return []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	}
}

func keypair(t *testing.T, dirPath, secretName, password string) {
	passFunc := func(_ bool) ([]byte, error) {
		return []byte(password), nil
	}

	keys, err := cosign.GenerateKeyPair(passFunc)
	assert.Nil(t, err)

	err = os.Mkdir(filepath.Join(dirPath, secretName), 0700)
	assert.Nil(t, err)

	privKeyPath := filepath.Join(dirPath, secretName, "cosign.key")
	err = ioutil.WriteFile(privKeyPath, keys.PrivateBytes, 0600)
	assert.Nil(t, err)

	pubKeyPath := filepath.Join(dirPath, secretName, "cosign.pub")
	err = ioutil.WriteFile(pubKeyPath, keys.PublicBytes, 0600)
	assert.Nil(t, err)

	passwordPath := filepath.Join(dirPath, secretName, "cosign.password")
	passwordBytes, _ := passFunc(true)
	err = ioutil.WriteFile(passwordPath, passwordBytes, 0600)
	assert.Nil(t, err)
}

func verify(keyRef, imageRef string, annotations map[string]interface{}) error {
	cmd := cli.VerifyCommand{
		KeyRef:      keyRef,
		Annotations: &annotations,
		CheckClaims: true,
	}

	args := []string{imageRef}

	return cmd.Exec(context.Background(), args)
}
