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

		signer = NewImageSigner(logger)
	)

	when("#Sign", func() {
		it("signs images", func() {
			// Create cosign.key
			secretLocation = t.TempDir()
			_, err := os.Create(fmt.Sprintf("%s/cosign.key", secretLocation))
			assert.Nil(t, err)

			// Create report.toml
			reportPath := fmt.Sprintf("%s/report.toml", secretLocation)
			reportFile, err := os.Create(reportPath)
			assert.Nil(t, err)
			_, err = reportFile.WriteString(`[image]\ntags = ["example-registry.io/test:latest", "example-registry.io/test:other-tag"]`)
			assert.Nil(t, err)

			cliSignCmdCallCount := 0
			// Mock out cliSignCmd
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, "example-registry.io/test:latest", imageRef)
				assert.Equal(t, fmt.Sprintf("%s/%s", secretLocation, "cosign.key"), ko.KeyRef)
				cliSignCmdCallCount++
				return nil
			}

			// Figure out if there is a way to ensure functinos were called
			// in my stub, create a file or something i can

			// Test Below
			err = signer.Sign(reportPath)
			assert.Nil(t, err)

			assert.Equal(t, 1, cliSignCmdCallCount)
		})
	})
}
