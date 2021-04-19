package registryfakes

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/pivotal/kpack/pkg/registry"
)

type keychainContainer struct {
	SecretRef registry.SecretRef
	Keychain  authn.Keychain
}

type FakeKeychainFactory struct {
	keychains []keychainContainer
}

func (f *FakeKeychainFactory) KeychainForSecretRef(ctx context.Context, secretRef registry.SecretRef) (authn.Keychain, error) {
	if keychain, ok := f.getKeychainForSecretRef(secretRef); ok {
		return keychain, nil
	}
	return nil, errors.Errorf("unable to find keychain for secret ref: %+v", secretRef)
}

func (f *FakeKeychainFactory) AddKeychainForSecretRef(t *testing.T, secretRef registry.SecretRef, keychain authn.Keychain) {
	t.Helper()

	if _, ok := f.getKeychainForSecretRef(secretRef); ok {
		t.Errorf("secret ref '%+v' already has a keychain", secretRef)
		return
	}

	f.keychains = append(f.keychains, keychainContainer{
		SecretRef: secretRef,
		Keychain:  keychain,
	})
}

func (f *FakeKeychainFactory) getKeychainForSecretRef(secretRef registry.SecretRef) (authn.Keychain, bool) {
	for _, item := range f.keychains {
		if equality.Semantic.DeepEqual(item.SecretRef, secretRef) {
			return item.Keychain, true
		}
	}
	return nil, false
}

type FakeKeychain struct {
	Name string
}

func (f *FakeKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}
