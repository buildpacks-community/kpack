package cosigner

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"github.com/tj/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			it("empty imageRef", func() {
				err := signer.Sign("", "")
				assert.NotNil(t, err)
			})

			it("wrong imageRef", func() {
				err := signer.Sign("invalidImage", "")
				assert.NotNil(t, err)
			})
		})

		when("invalid key", func() {

		})

		it("signs image", func() {
			// Todo: how do we mock kube secrets

			secretObject := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ignored-secret",
					Namespace: "namespace",
				},
				StringData: map[string]string{
					"username": "username",
					"password": "password",
				},
				Type: corev1.SecretTypeBasicAuth,
			}
			fakeK8sClient := fake.NewSimpleClientset(secretObject)

			assert.NotNil(t, fakeK8sClient)

			signer.Sign(imageRef, "")

			// Populate registry with an image
			// - Same as notary
			// Call cosign function to sign that image
			// - SignCmd
			// - Verify that there is no error when cosign.Sign
		})
	})

	// Test using service account

	// Test signing builder and other resources
}
