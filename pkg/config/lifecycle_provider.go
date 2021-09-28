package config

import (
	"context"
	"sync/atomic"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/registry"
)

const (
	LifecycleConfigName        = "lifecycle-image"
	LifecycleConfigKey         = "image"
	ServiceAccountNameKey      = "serviceAccountRef.name"
	ServiceAccountNamespaceKey = "serviceAccountRef.namespace"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
}

type lifecycleData struct {
	image v1.Image
	err   error
}

type LifecycleProvider struct {
	registryClient  RegistryClient
	keychainFactory registry.KeychainFactory
	lifecycleData   atomic.Value
	handlers        []func()
}

func NewLifecycleProvider(client RegistryClient, keychainFactory registry.KeychainFactory) *LifecycleProvider {
	return &LifecycleProvider{
		registryClient:  client,
		keychainFactory: keychainFactory,
	}
}

func (l *LifecycleProvider) UpdateImage(cm *corev1.ConfigMap) {
	data, shouldCallHandlers := l.updateImage(context.Background(), cm)
	if shouldCallHandlers {
		l.callHandlers()
	}
	l.lifecycleData.Store(data)
}

func (l *LifecycleProvider) updateImage(ctx context.Context, cm *corev1.ConfigMap) (*lifecycleData, bool) {
	data := &lifecycleData{}
	imageRef, ok := cm.Data[LifecycleConfigKey]
	if !ok {
		data.err = errors.Errorf("%s config invalid", LifecycleConfigName)
		return data, true
	}

	keychain, err := l.keychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		ServiceAccount: cm.Data[ServiceAccountNameKey],
		Namespace:      cm.Data[ServiceAccountNamespaceKey],
	})
	if err != nil {
		data.err = errors.Wrapf(err, "fetching keychain to read lifecycle")
		return data, true
	}

	l.fetchImage(keychain, imageRef, data)
	if data.err != nil {
		return data, true
	}

	// Don't care if old image errored
	oldImg, _ := l.GetImage()
	var isNewImg bool
	isNewImg, data.err = isNewImage(oldImg, data.image)
	return data, isNewImg
}

func (l *LifecycleProvider) GetImage() (v1.Image, error) {
	d, ok := l.lifecycleData.Load().(*lifecycleData)
	if !ok {
		return nil, errors.New("lifecycle image has not been loaded")
	}

	return d.image, d.err
}

func (l *LifecycleProvider) AddEventHandler(handler func()) {
	l.handlers = append(l.handlers, handler)
}

func (l *LifecycleProvider) fetchImage(keychain authn.Keychain, imageRef string, data *lifecycleData) {
	img, _, err := l.registryClient.Fetch(keychain, imageRef)
	if err != nil {
		data.err = errors.Wrap(err, "failed to fetch lifecycle image")
		return
	}
	data.image = img
}

func isNewImage(oldImg v1.Image, newImg v1.Image) (bool, error) {
	if oldImg == nil {
		return true, nil
	}

	d0, err := oldImg.Digest()
	if err != nil {
		return true, err
	}

	d1, err := newImg.Digest()
	if err != nil {
		return true, err
	}

	return d0 != d1, nil
}

func (l *LifecycleProvider) callHandlers() {
	for _, cb := range l.handlers {
		cb()
	}
}
