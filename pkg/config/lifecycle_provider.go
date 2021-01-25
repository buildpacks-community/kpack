package config

import (
	"sync/atomic"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

const (
	LifecycleConfigName = "lifecycle-image"
	LifecycleConfigKey  = "image"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
}

type lifecycleData struct {
	image v1.Image
	err   error
}

type LifecycleProvider struct {
	RegistryClient RegistryClient
	Keychain       authn.Keychain
	lifecycleData  atomic.Value
	callbacks      []func()
}

func NewLifecycleProvider(lifecycleImageRef string, client RegistryClient, keychain authn.Keychain) *LifecycleProvider {
	p := &LifecycleProvider{
		RegistryClient: client,
		Keychain:       keychain,
	}

	data := &lifecycleData{}

	data.image, data.err = p.fetchImage(lifecycleImageRef)

	p.lifecycleData.Store(data)
	return p
}

func (l *LifecycleProvider) UpdateImage(cm *corev1.ConfigMap) {
	data := &lifecycleData{}
	defer l.callCallBacks()
	defer l.lifecycleData.Store(data)

	imageRef, ok := cm.Data[LifecycleConfigKey]
	if !ok {
		data.err = errors.New("lifecycle-image config invalid")
		return
	}

	data.image, data.err = l.fetchImage(imageRef)
}

func (l *LifecycleProvider) GetImage() (v1.Image, error) {
	d, ok := l.lifecycleData.Load().(*lifecycleData)
	if !ok {
		return nil, errors.New("lifecycle image has not been loaded")
	}

	if d.err != nil {
		return nil, d.err
	}

	return d.image, nil
}

func (l *LifecycleProvider) RegisterCallback(callback func()) {
	l.callbacks = append(l.callbacks, callback)
}

func (l *LifecycleProvider) fetchImage(imageRef string) (v1.Image, error) {
	img, _, err := l.RegistryClient.Fetch(l.Keychain, imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch lifecycle image")
	}
	return img, nil
}

func (l *LifecycleProvider) callCallBacks() {
	for _, cb := range l.callbacks {
		cb()
	}
}
