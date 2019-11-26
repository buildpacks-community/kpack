package testhelpers

import (
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/pivotal/kpack/pkg/registry"
)

type FakeKeychainFactory struct {
	SecretRef registry.SecretRef
}

func (f *FakeKeychainFactory) KeychainForSecretRef(secretRef registry.SecretRef) (authn.Keychain, error) {
	f.SecretRef = secretRef
	return &fakeKeychain{}, nil
}

type fakeKeychain struct {
}

func (f *fakeKeychain) Resolve(authn.Resource) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}
