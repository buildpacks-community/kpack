package dockercreds

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pivotal/kpack/pkg/registry"
)

type cacheKey string

type cachedKeychainFactory struct {
	keychainFactory registry.KeychainFactory
	cache           map[cacheKey]authn.Keychain
}

func NewCachedKeychainFactory(keychainFactory registry.KeychainFactory) registry.KeychainFactory {
	return &cachedKeychainFactory{
		keychainFactory: keychainFactory,
		cache:           make(map[cacheKey]authn.Keychain),
	}
}

func (f *cachedKeychainFactory) KeychainForSecretRef(ctx context.Context, secretRef registry.SecretRef) (authn.Keychain, error) {
	key := makeKey(secretRef)
	if keychain, found := f.cache[key]; found {
		return keychain, nil
	}

	keychain, err := f.keychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		return nil, err
	}

	f.cache[key] = keychain
	return keychain, nil
}

func makeKey(secretRef registry.SecretRef) cacheKey {
	return cacheKey(fmt.Sprintf("%v/%v", secretRef.Namespace, secretRef.ServiceAccount))
}
