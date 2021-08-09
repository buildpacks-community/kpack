package cosigner

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"testing"

	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/cmd/cosign/cli"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Cosign Image Signer Main", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		logger = log.New(ioutil.Discard, "", 0)

		// Todo: evaluate if not being able to test secrets is an issue
		// Cosign does not allow us to set the k8sclient, therefore, how do we confirm that it will be pointing to the correct thing
		// It will likely use the default kubeconfig? is the kubeconfig set as default for kpack then?

		k8sSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testSecretName",
				Namespace: "testNamespace",
			},
			Data: map[string][]byte{},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "testNamespace",
				Name:      "serviceAccountName",
			},
			Secrets: []corev1.ObjectReference{
				{
					Kind: "secret",
					Name: "testSecretName",
				},
			},
		}

		fakeK8sClient = fake.NewSimpleClientset(serviceAccount, k8sSecret)

		signer = NewImageSigner(logger, fakeK8sClient)
	)

	when("#Sign", func() {
		var (
			imageRef string
		)

		it.Before(func() {
			imageRef = "example-registry.io/test@1.0"
		})

		when("invalid imageRef", func() {
			it.Before(func() {
				cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
					return errors.New("fake cli.SignCmd error")
				}
			})

			it("should error", func() {
				err := signer.Sign("", "testNamespace", "serviceAccountName")
				assert.EqualError(t, err, "signing reference image is empty")

				err = signer.Sign("invalidImage", "testNamespace", "serviceAccountName")
				assert.EqualError(t, err, "signing: fake cli.SignCmd error")
			})
		})

		when("invalid serviceaccount", func() {
			it("should error", func() {
				err := signer.Sign("fakeImage", "", "serviceAccountName")
				assert.EqualError(t, err, "namespace is empty")

				err = signer.Sign(imageRef, "fakeNamespace", "serviceAccountName")
				assert.EqualError(t, err, "get service account: serviceaccounts \"serviceAccountName\" not found")
			})
		})

		when("invalid serviceaccount", func() {
			it("should error", func() {
				err := signer.Sign("fakeImage", "testNamespace", "")
				assert.EqualError(t, err, "service account name is empty")

				err = signer.Sign(imageRef, "testNamespace", "fakeServiceAccountName")
				assert.EqualError(t, err, "get service account: serviceaccounts \"fakeServiceAccountName\" not found")
			})
		})

		// Todo: Iteration 4: Update to sign builder and other resources

		it("signs images", func() {
			imageRef = "registry.example.com/fakeProject/fakeImage:test"
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRefActual, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, imageRef, imageRefActual)
				assert.Equal(t, fmt.Sprintf("%s/%s", k8sSecret.ObjectMeta.Namespace, k8sSecret.ObjectMeta.Name), ko.KeyRef)
				return nil
			}

			err := signer.Sign(imageRef, "testNamespace", "serviceAccountName")
			assert.Nil(t, err)
		})
	})
}
