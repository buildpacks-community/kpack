package dockercreds

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
)

func TestCachedKeychainFactory(t *testing.T) {
	spec.Run(t, "CachedSecretKeychainFactory", testCachedKeychainFactory)
}

func testCachedKeychainFactory(t *testing.T, when spec.G, it spec.S) {
	var (
		ctx              = context.Background()
		expectedKeychain = authn.NewMultiKeychain(authn.DefaultKeychain)
		baseFactory      *fakeFactory
		factory          registry.KeychainFactory
	)
	it.Before(func() {
		baseFactory = &fakeFactory{keychain: expectedKeychain, err: nil}
		factory = NewCachedKeychainFactory(baseFactory)
	})

	it("returns the correct keychain", func() {
		ref := registry.SecretRef{}
		keychain, err := factory.KeychainForSecretRef(ctx, ref)
		assert.NoError(t, err)
		assert.Equal(t, expectedKeychain, keychain)

		assert.Len(t, baseFactory.argsForCall, 1)
		assert.Equal(t, ref, baseFactory.argsForCall[0])
	})

	it("caches results", func() {
		ref := registry.SecretRef{
			ServiceAccount: "some-service-account",
			Namespace:      "some-namespace",
		}
		keychain, err := factory.KeychainForSecretRef(ctx, ref)
		assert.NoError(t, err)
		assert.Equal(t, expectedKeychain, keychain)

		keychain, err = factory.KeychainForSecretRef(ctx, ref)
		assert.NoError(t, err)
		assert.Equal(t, expectedKeychain, keychain)

		assert.Len(t, baseFactory.argsForCall, 1)
		assert.Equal(t, ref, baseFactory.argsForCall[0])
	})

	it("can handle multiple keychains", func() {
		ref1 := registry.SecretRef{
			ServiceAccount: "some-service-account",
			Namespace:      "some-namespace",
		}
		keychain, err := factory.KeychainForSecretRef(ctx, ref1)
		assert.NoError(t, err)
		assert.Equal(t, expectedKeychain, keychain)

		ref2 := registry.SecretRef{
			ServiceAccount: "some-other-service-account",
			Namespace:      "some-other-namespace",
		}
		keychain, err = factory.KeychainForSecretRef(ctx, ref2)
		assert.NoError(t, err)
		assert.Equal(t, expectedKeychain, keychain)

		assert.Len(t, baseFactory.argsForCall, 2)
		assert.Equal(t, ref1, baseFactory.argsForCall[0])
		assert.Equal(t, ref2, baseFactory.argsForCall[1])
	})
}

type fakeFactory struct {
	keychain    authn.Keychain
	err         error
	argsForCall []registry.SecretRef
}

func (f *fakeFactory) KeychainForSecretRef(_ context.Context, ref registry.SecretRef) (authn.Keychain, error) {
	f.argsForCall = append(f.argsForCall, ref)
	return f.keychain, f.err
}
