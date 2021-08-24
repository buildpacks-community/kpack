package cosigner

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/cmd/cosign/cli"
	"github.com/stretchr/testify/assert"
)

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Cosign Image Signer Main", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		logger = log.New(ioutil.Discard, "", 0)

		signer     = NewImageSigner(logger)
		reportPath string
	)

	when("#Sign", func() {
		it.Before(func() {
			// Override secretLocation for test
			secretLocation = createCosignKeyFiles(t)

			reportPath = createReportToml(t, secretLocation)
		})

		it("signs images", func() {
			cliSignCmdCallCount := 0
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, "example-registry.io/test:latest", imageRef)
				assert.Contains(t, ko.KeyRef, "cosign.key")
				assert.Contains(t, ko.KeyRef, secretLocation)
				assert.Nil(t, annotations)
				cliSignCmdCallCount++
				return nil
			}

			err := signer.Sign(reportPath, nil)
			assert.Nil(t, err)

			assert.Equal(t, 2, cliSignCmdCallCount)
		})

		it("signs with annotations", func() {
			expectedAnnotation := map[string]interface{}{
				"annotationKey1": "value1",
			}

			cliSignCmdCallCount := 0
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, "example-registry.io/test:latest", imageRef)
				assert.Contains(t, ko.KeyRef, "cosign.key")
				assert.Contains(t, ko.KeyRef, secretLocation)
				assert.Equal(t, expectedAnnotation, annotations)
				cliSignCmdCallCount++
				return nil
			}

			err := signer.Sign(reportPath, expectedAnnotation)
			assert.Nil(t, err)

			assert.Equal(t, 2, cliSignCmdCallCount)
		})
	})
}

func createCosignKeyFiles(t *testing.T) string {
	secretLocation = t.TempDir()

	os.Mkdir(fmt.Sprintf("%s/secret-name-1", secretLocation), 0700)
	os.Mkdir(fmt.Sprintf("%s/secret-name-2", secretLocation), 0700)

	os.Create(fmt.Sprintf("%s/secret-name-1/cosign.key", secretLocation))
	os.Create(fmt.Sprintf("%s/secret-name-2/cosign.key", secretLocation))

	return secretLocation
}

func createReportToml(t *testing.T, secretLocation string) string {
	reportPath := fmt.Sprintf("%s/report.toml", secretLocation)
	reportFile, err := os.Create(reportPath)
	assert.Nil(t, err)
	_, err = reportFile.WriteString(`[image]
		tags = ["example-registry.io/test:latest", "example-registry.io/test:other-tag"]`)
	assert.Nil(t, err)

	return reportPath
}
