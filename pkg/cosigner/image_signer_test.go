package cosigner

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/cmd/cosign/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Cosign Image Signer Main", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		logger = log.New(ioutil.Discard, "", 0)

		client = registryfakes.NewFakeClient()

		signer = ImageSigner{
			Logger: logger,
			Client: client,
		}
	)

	// Test cosign signing
	when("#Sign", func() {
		var (
			keychain authn.Keychain
			image    v1.Image
			imageRef string
		)

		it.Before(func() {
			keychain = &registryfakes.FakeKeychain{}
			imageRef = "example-registry.io/test@1.0"

			var err error
			image, err = random.Image(0, 0)
			require.NoError(t, err)

			client.AddImage(imageRef, image, keychain)
		})

		// Error when signing with invalid key
		// Error when signing with invalid image
		// Success when signing with valid image/key
		when("invalid imageRef", func() {
			it.Before(func() {
				cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					return errors.New("fake cli.SignCmd error")
				}
			})

			it("should error", func() {
				err := signer.Sign("", "fakeKey")
				assert.EqualError(t, err, "signing reference image is empty")

				err = signer.Sign("invalidImage", "fakeKey")
				assert.EqualError(t, err, "signing: fake cli.SignCmd error")
			})
		})

		when("invalid keyPath", func() {
			it.Before(func() {
				cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					return errors.New("fake cli.SignCmd error")
				}
			})

			it("should error", func() {
				err := signer.Sign("fakeImage", "")
				assert.EqualError(t, err, "signing key path is empty")

				err = signer.Sign("fakeImage", "invalidKey")
				assert.EqualError(t, err, "signing: fake cli.SignCmd error")
			})
		})

		// Todo: Iteration 1: Make a signing test using keyless or local keys
		// Todo: Iteration 2: Update to use secrets
		// - Accept that cosign will work if our string is correct
		// - Make a PR to cosign to abstract the kubernetes client
		// Todo: Iteration 3: Update to use service account secrets
		// Todo: Iteration 4: Update to sign builder and other resources

		// Issues
		// How to mock secrets for cosign to consume?
		//   (Make mock kube server and set the kubeconfig?)
		// How to mock registry for cosign to sign to
		//   Verify that an image was then signed

		it("signs images", func() {
			imageRef = "registry.example.com/fakeProject/fakeImage:test"

			privKeyPath := "privateKeyPath/path"

			// Mock cliSignCmd to verify passed in variables
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRefActual, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, imageRefActual, imageRef)
				assert.Equal(t, ko.KeyRef, privKeyPath)
				return nil
			}

			err := signer.Sign(imageRef, privKeyPath)
			assert.Nil(t, err)
		})

		// Obtaining secrets from service account
		// - Update signer.Sign to only take signer.Sign(imageRef, serviceAccountName)
		// - Look at all service accounts
		//   - Get all cosign secrets
		//
	})

}
