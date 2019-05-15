package registry_test

import (
	"encoding/base64"
	"fmt"
	"github.com/pivotal/build-service-system/pkg/registry"
	"k8s.io/apimachinery/pkg/util/uuid"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretKeychainFactory(t *testing.T) {
	spec.Run(t, "SecretKeychainFactory", testSecretKeychain)
}

func testSecretKeychain(t *testing.T, when spec.G, it spec.S) {
	const (
		serviceAccountName = "some-service-account"
	)

	var (
		fakeClient *fake.Clientset

		testNamespace = "namespace"

		Expect          func(interface{}, ...interface{}) GomegaAssertion
		keychainFactory *registry.SecretKeychainFactory
	)

	it.Before(func() {
		Expect = NewGomegaWithT(t).Expect
		fakeClient = fake.NewSimpleClientset(&v1.Secret{})
	})

	when("SecretKeychainFactory", func() {
		it.Before(func() {
			secretMgr := &registry.SecretManager{Client: fakeClient.CoreV1()}
			keychainFactory = &registry.SecretKeychainFactory{secretMgr}

			err := saveSecrets(fakeClient.CoreV1(), testNamespace, serviceAccountName,
				[]registry.RegistryUser{
					registry.NewRegistryUser("https://godoker.reg.com", "foobar", "foobar321"),
					registry.NewRegistryUser("https://redhook.port", "brooklyn", "nothip"),
				})
			Expect(err).NotTo(HaveOccurred())
		})

		when("#NewImage", func() {
			it("returns a keychain that provides auth credentials", func() {
				keychain := keychainFactory.KeychainForImageRef(&fakeImageRef{serviceAccountName: serviceAccountName, namespace: testNamespace})

				reference, err := name.ParseReference("redhook.port/name", name.WeakValidation)
				Expect(err).NotTo(HaveOccurred())

				authenticator, err := keychain.Resolve(reference.Context().Registry)
				Expect(err).NotTo(HaveOccurred())

				encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "brooklyn", "nothip")))
				value := fmt.Sprintf("Basic %s", encoded)

				Expect(authenticator.Authorization()).To(Equal(value))
			})

			it("returns an error if no credentials are provided for the registry", func() {
				keychain := keychainFactory.KeychainForImageRef(&fakeImageRef{serviceAccountName: serviceAccountName, namespace: testNamespace})

				reference, err := name.ParseReference("notareal.reg/name", name.WeakValidation)
				Expect(err).NotTo(HaveOccurred())

				_, err = keychain.Resolve(reference.Context().Registry)
				Expect(err).To(MatchError("credentials not found for: notareal.reg"))

			})

			when("service account is empty", func() {
				it("returns anonymous auth", func() {
					keychain := keychainFactory.KeychainForImageRef(&fakeImageRef{serviceAccountName: "", namespace: testNamespace})

					reference, err := name.ParseReference("notareal.reg/name", name.WeakValidation)
					Expect(err).NotTo(HaveOccurred())

					authenticator, err := keychain.Resolve(reference.Context().Registry)
					Expect(err).NotTo(HaveOccurred())

					Expect(authenticator).To(Equal(authn.Anonymous))
				})
			})
		})
	})
}

func saveSecrets(coreV1 v12.CoreV1Interface, namespace, serviceAccount string, users []registry.RegistryUser) error {
	secrets := []v1.ObjectReference{}

	for _, user := range users {
		secret, err := coreV1.Secrets(namespace).Create(&v1.Secret{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: string(uuid.NewUUID()),
				Annotations: map[string]string{
					registry.KnativeRegistryUrl: user.URL,
				},
			},
			Data: map[string][]byte{
				"username": []byte(user.Username),
				"password": []byte(user.Password),
			},
			Type: v1.SecretTypeBasicAuth,
		})
		if err != nil {
			return err
		}

		secrets = append(secrets, v1.ObjectReference{
			Name: secret.Name,
		})
	}

	_, err := coreV1.ServiceAccounts(namespace).Create(&v1.ServiceAccount{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: serviceAccount,
		},
		Secrets: secrets,
	})
	return err
}

type fakeImageRef struct {
	serviceAccountName string
	namespace          string
}

func (f *fakeImageRef) Namespace() string {
	return f.namespace
}

func (f *fakeImageRef) RepoName() string {
	return "NOT-NEEDED"
}

func (f *fakeImageRef) ServiceAccount() string {
	return f.serviceAccountName
}
