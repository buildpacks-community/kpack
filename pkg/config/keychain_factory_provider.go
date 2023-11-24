package config

import (
	"github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pkg/errors"
	k8sclient "k8s.io/client-go/kubernetes"
	"sync/atomic"
)

type dynamicKeychainFactoryProvider struct {
	client              k8sclient.Interface
	keychainFactoryData atomic.Value
	handlers            []func()
}

func NewKeychainFactoryProvider(client k8sclient.Interface) *dynamicKeychainFactoryProvider {
	return &dynamicKeychainFactoryProvider{
		client: client,
	}
}

func (k *dynamicKeychainFactoryProvider) KeychainFactory() (registry.KeychainFactory, error) {
	d, ok := k.keychainFactoryData.Load().(keychainFactoryStore)
	if !ok {
		return nil, errors.Errorf("Error: ")
	}

	return d.keychainFactory, d.err
}

func (k *dynamicKeychainFactoryProvider) UpdateKeychainFactory() error {
	keychainFactory := k.keychainFactoryData.Load().(keychainFactoryStore).keychainFactory
	if keychainFactory == nil {
		keychainFactory, err := k8sdockercreds.NewSecretKeychainFactory(k.client)
		if err != nil {
			k.keychainFactoryData.Store(keychainFactoryStore{err: err})
		}
		k.keychainFactoryData.Store(keychainFactoryStore{keychainFactory: keychainFactory})
		return err
	} else {
		return nil
	}
}

type keychainFactoryStore struct {
	keychainFactory registry.KeychainFactory
	err             error
}

type KeychainFactoryProvider interface {
	KeychainFactory() (registry.KeychainFactory, error)
}
