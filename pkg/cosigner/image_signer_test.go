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

var passFunc = func(_ bool) ([]byte, error) {
	return []byte(""), nil
}

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Cosign Image Signer Main", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		report  lifecycle.ExportReport
		reader  *os.File
		writer  *os.File
		scanner *bufio.Scanner
	)

	it.Before(func() {
		scanner, reader, writer = mockLogger(t)
	})

	it.After(func() {
		resetLogger(reader, writer)
	})

	when("#Sign", func() {
		when("signing occurs", func() {
			it.Before(func() {
				// Override secretLocation for test
				secretLocation = createCosignKeyFiles(t)

				report = createReportToml(t)

				os.Unsetenv(cosignRepositoryEnv)
				os.Unsetenv(cosignDockerMediaTypesEnv)
			})

			it("signs images", func() {
				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, "example-registry.io/test:latest", imageRef)
					assert.Contains(t, ko.KeyRef, "cosign.key")
					assert.Contains(t, ko.KeyRef, secretLocation)

					password, err := ko.PassFunc(true)
					assert.Nil(t, err)

					assert.Equal(t, []byte(""), password)
					assert.Nil(t, annotations)
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 2, cliSignCmdCallCount)
			})

			it("signs images with key password", func() {
				firstPassword := []byte("secretPass1")
				secondPassword := []byte("secretPass2")
				ioutil.WriteFile(fmt.Sprintf("%s/secret-name-1/cosign.password", secretLocation), firstPassword, 0644)
				ioutil.WriteFile(fmt.Sprintf("%s/secret-name-2/cosign.password", secretLocation), secondPassword, 0644)

				cliSignCmdCallCount := 0
				firstPasswordSeenCount := 0
				secondPasswordSeenCount := 0

				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, "example-registry.io/test:latest", imageRef)
					assert.Contains(t, ko.KeyRef, "cosign.key")
					assert.Contains(t, ko.KeyRef, secretLocation)

					password, err := ko.PassFunc(true)
					assert.Nil(t, err)

					assert.Contains(t, [][]byte{firstPassword, secondPassword}, password)

					if string(firstPassword) == string(password) {
						firstPasswordSeenCount++
					}

					if string(secondPassword) == string(password) {
						secondPasswordSeenCount++
					}

					assert.Nil(t, annotations)
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 2, cliSignCmdCallCount)
				assert.Equal(t, 1, firstPasswordSeenCount)
				assert.Equal(t, 1, secondPasswordSeenCount)
			})

			it("signs with annotations", func() {
				expectedAnnotation := map[string]interface{}{
					"annotationKey1": "value1",
				}

				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					assert.Equal(t, "example-registry.io/test:latest", imageRef)
					assert.Contains(t, ko.KeyRef, "cosign.key")
					assert.Contains(t, ko.KeyRef, secretLocation)
					assert.Equal(t, expectedAnnotation, annotations)
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, expectedAnnotation, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 2, cliSignCmdCallCount)
			})

			it("errors when signing fails", func() {
				cliSignCmdCallCount := 0

				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					cliSignCmdCallCount++
					return fmt.Errorf("fake error")
				}

				expectedErrorMessage := fmt.Sprintf("unable to sign image with %s/secret-name-1/cosign.key: fake error", secretLocation)

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, nil, nil, nil)
				assert.Error(t, err)
				assert.Equal(t, expectedErrorMessage, err.Error())
				assert.Equal(t, 1, cliSignCmdCallCount)
			})

			it("sets COSIGN_REPOSITORY environment variable", func() {
				cliSignCmdCallCount := 0

				assert.Empty(t, len(os.Getenv(cosignRepositoryEnv)))
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					if strings.Contains(ko.KeyRef, "secret-name-1") {
						assert.Equal(t, "registry.example.com/fakeproject", os.Getenv(cosignRepositoryEnv))
					} else {
						assertUnset(t, cosignRepositoryEnv)
					}

					cliSignCmdCallCount++
					return nil
				}

				cosignRepositories := map[string]interface{}{
					"secret-name-1": "registry.example.com/fakeproject",
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, nil, cosignRepositories, nil)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, cosignRepositoryEnv)
			})

			it("sets COSIGN_DOCKER_MEDIA_TYPES environment variable", func() {
				cliSignCmdCallCount := 0

				assertUnset(t, cosignDockerMediaTypesEnv)
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
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
				err := signer.Sign(report, nil, nil, cosignDockerMediaTypes)
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
				err := signer.Sign(report, nil, cosignRepositories, cosignDockerMediaTypes)
				assert.Nil(t, err)
				assert.Equal(t, 2, cliSignCmdCallCount)

				assertUnset(t, cosignDockerMediaTypesEnv)
				assertUnset(t, cosignRepositoryEnv)
			})
		})

		when("signing is skipped because", func() {
			it("has no cosign secrets", func() {
				secretLocation = t.TempDir()
				report = createReportToml(t)

				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 0, cliSignCmdCallCount)

				assert.Equal(t, "no keys found for cosign signing", logText(scanner))
			})

			it("has invalid directory", func() {
				secretLocation = "/fake/location/that/doesnt/exist"
				report = createReportToml(t)

				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 0, cliSignCmdCallCount)

				assert.Equal(t, "no keys found for cosign signing: open /fake/location/that/doesnt/exist: no such file or directory", logText(scanner))
			})

			it("has no images in report", func() {
				secretLocation = createCosignKeyFiles(t)
				report = createEmptyReportToml(t)

				cliSignCmdCallCount := 0
				cliSignCmd := func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					t.Helper()
					cliSignCmdCallCount++
					return nil
				}

				signer := NewImageSigner(log.New(writer, "", 0), cliSignCmd)
				err := signer.Sign(report, nil, nil, nil)
				assert.Nil(t, err)

				assert.Equal(t, 0, cliSignCmdCallCount)

				assert.Equal(t, "no image tag to sign", logText(scanner))
			})
		})
	})

	when("#Cosign.SignCmd", func() {
		it("signs an image", func() {
			secretLocation = t.TempDir()

			repo, stop := reg(t)
			defer stop()

			imgName := path.Join(repo, "cosign-e2e")

			_, _, cleanup := mkimage(t, imgName)
			defer cleanup()

			keypair(t, secretLocation, "secret-name-1")
			privKeyPath := path.Join(secretLocation, "secret-name-1", "cosign.key")
			pubKeyPath := path.Join(secretLocation, "secret-name-1", "cosign.pub")

			ctx := context.Background()
			// Verify+download should fail at first
			err := verify(pubKeyPath, imgName, nil)
			assert.NotNil(t, err)
			err = download.SignatureCmd(ctx, imgName)
			assert.NotNil(t, err)

			// Sign
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

	keypair(t, dirPath, "secret-name-1")
	keypair(t, dirPath, "secret-name-2")

	return dirPath
}

func createReportToml(t *testing.T) lifecycle.ExportReport {
	var r lifecycle.ExportReport
	_, err := toml.Decode(`[image]
	tags = ["example-registry.io/test:latest", "example-registry.io/test:other-tag"]`, &r)
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

func mkimage(t *testing.T, n string) (name.Reference, *remote.Descriptor, func()) {
	ref, err := name.ParseReference(n, name.WeakValidation)
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

	return ref, remoteImage, cleanup
}

func registryClientOpts(ctx context.Context) []remote.Option {
	return []remote.Option{
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithContext(ctx),
	}
}

func keypair(t *testing.T, dirPath, secretName string) {
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
	}

	args := []string{imageRef}

	return cmd.Exec(context.Background(), args)
}
