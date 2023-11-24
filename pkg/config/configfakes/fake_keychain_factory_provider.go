package configfakes

import (
	"github.com/pivotal/kpack/pkg/registry"
)

type FakeKeychainFactoryProvider struct {
	keychainFactory registry.KeychainFactory
}

func (f *FakeKeychainFactoryProvider) AddKeychainFactory(factory registry.KeychainFactory) error {
	f.keychainFactory = factory
	return nil
}

func (f *FakeKeychainFactoryProvider) KeychainFactory() (registry.KeychainFactory, error) {
	return f.keychainFactory, nil
}
