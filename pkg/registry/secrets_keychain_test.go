package registry_test

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/build-service-system/pkg/registry"
	"github.com/pivotal/build-service-system/pkg/secret"
	secrethelper "github.com/pivotal/build-service-system/pkg/secret/testhelpers"
)

func TestSecretKeychainFactory(t *testing.T) {
	spec.Run(t, "SecretKeychainFactory", testSecretKeychain)
}

func testSecretKeychain(t *testing.T, when spec.G, it spec.S) {
	const (
		serviceAccountName = "some-service-account"
	)

	var (
		testNamespace   = "namespace"
		keychainFactory *registry.SecretKeychainFactory
		fakeClient      = fake.NewSimpleClientset(&v1.Secret{})
	)

	when("SecretKeychainFactory", func() {
		it.Before(func() {
			keychainFactory = registry.NewSecretKeychainFactory(fakeClient)

			err := secrethelper.SaveDockerSecrets(fakeClient, testNamespace, serviceAccountName,
				[]secret.URLAndUser{
					secret.NewURLAndUser("https://godoker.reg.com", "foobar", "foobar321"),
					secret.NewURLAndUser("https://redhook.port", "brooklyn", "nothip"),
				})
			assert.NoError(t, err)
		})

		when("#NewImage", func() {
			it("returns a keychain that provides auth credentials", func() {
				keychain := keychainFactory.KeychainForImageRef(&fakeImageRef{serviceAccountName: serviceAccountName, namespace: testNamespace})

				reference, err := name.ParseReference("redhook.port/name", name.WeakValidation)
				assert.NoError(t, err)

				authenticator, err := keychain.Resolve(reference.Context().Registry)
				assert.NoError(t, err)

				encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", "brooklyn", "nothip")))

				auth, err := authenticator.Authorization()
				assert.NoError(t, err)
				assert.Equal(t, auth, fmt.Sprintf("Basic %s", encoded))
			})

			it("returns an error if no credentials are provided for the registry", func() {
				keychain := keychainFactory.KeychainForImageRef(&fakeImageRef{serviceAccountName: serviceAccountName, namespace: testNamespace})

				reference, err := name.ParseReference("notareal.reg/name", name.WeakValidation)
				assert.NoError(t, err)

				_, err = keychain.Resolve(reference.Context().Registry)
				assert.Error(t, err, "credentials not found for: notareal.reg")

			})

			when("service account is empty", func() {
				it("returns anonymous auth", func() {
					keychain := keychainFactory.KeychainForImageRef(&fakeImageRef{serviceAccountName: "", namespace: testNamespace})

					reference, err := name.ParseReference("notareal.reg/name", name.WeakValidation)
					assert.NoError(t, err)

					authenticator, err := keychain.Resolve(reference.Context().Registry)
					assert.NoError(t, err)

					assert.Equal(t, authenticator, authn.Anonymous)
				})
			})
		})
	})
}

type fakeImageRef struct {
	serviceAccountName string
	namespace          string
}

func (f *fakeImageRef) Namespace() string {
	return f.namespace
}

func (f *fakeImageRef) Tag() string {
	return "NOT-NEEDED"
}

func (f *fakeImageRef) ServiceAccount() string {
	return f.serviceAccountName
}
