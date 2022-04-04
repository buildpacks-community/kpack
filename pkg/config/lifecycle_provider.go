package config

import (
	"context"
	"sync/atomic"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	LifecycleConfigName        = "lifecycle-image"
	LifecycleConfigKey         = "image"
	serviceAccountNameKey      = "serviceAccountRef.name"
	serviceAccountNamespaceKey = "serviceAccountRef.namespace"
	lifecycleMetadataLabel     = "io.buildpacks.lifecycle.metadata"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
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

func (l *LifecycleProvider) LayerForOS(os string) (v1.Layer, cnb.LifecycleMetadata, error) {
	lifecycle, err := l.lifecycle()
	if err != nil {
		return nil, cnb.LifecycleMetadata{}, err
	}

	switch os {
	case "linux":
		layer, err := lifecycle.linux.toLazyLayer(lifecycle.keychain)
		return layer, lifecycle.metadata, err
	case "windows":
		layer, err := lifecycle.windows.toLazyLayer(lifecycle.keychain)
		return layer, lifecycle.metadata, err
	default:
		return nil, cnb.LifecycleMetadata{}, errors.Errorf("unrecognized os %s", os)
	}
}

func (l *LifecycleProvider) UpdateImage(cm *corev1.ConfigMap) {
	lifecycle, err := l.read(context.Background(), cm)
	if err != nil {
		l.lifecycleData.Store(configmapRead{err: err})
		return
	}

	if l.isNewImage(lifecycle) {
		l.callHandlers()
	}
	l.lifecycleData.Store(configmapRead{lifecycle: lifecycle})
}

func (l *LifecycleProvider) AddEventHandler(handler func()) {
	l.handlers = append(l.handlers, handler)
}

func (l *LifecycleProvider) read(ctx context.Context, cm *corev1.ConfigMap) (*lifecycle, error) {
	imageRef, ok := cm.Data[LifecycleConfigKey]
	if !ok {
		return nil, errors.Errorf("%s config invalid", LifecycleConfigName)
	}

	keychain, err := l.keychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		ServiceAccount: cm.Data[serviceAccountNameKey],
		Namespace:      cm.Data[serviceAccountNamespaceKey],
	})
	if err != nil {
		return nil, errors.Wrapf(err, "fetching keychain to read lifecycle")
	}

	img, _, err := l.registryClient.Fetch(keychain, imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch lifecycle image")
	}

	lifecycleMd := cnb.LifecycleMetadata{}
	err = imagehelpers.GetLabel(img, lifecycleMetadataLabel, &lifecycleMd)
	if err != nil {
		return nil, err
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}

	linuxLayer, err := lifecycleLayerForOS(imageRef, img, "linux")
	if err != nil {
		return nil, err
	}

	windowsLayer, err := lifecycleLayerForOS(imageRef, img, "windows")
	if err != nil {
		return nil, err
	}

	return &lifecycle{
		keychain: keychain,
		digest:   digest,
		metadata: lifecycleMd,
		linux:    linuxLayer,
		windows:  windowsLayer,
	}, nil
}

func lifecycleLayerForOS(imageRef string, image v1.Image, os string) (*lifecycleLayer, error) {
	diffId, err := imagehelpers.GetStringLabel(image, os)
	if err != nil {
		return nil, errors.Wrapf(err, "could not find lifecycle for os: %s", os)
	}

	diffID, err := v1.NewHash(diffId)
	if err != nil {
		return nil, err
	}

	layer, err := image.LayerByDiffID(diffID)
	if err != nil {
		return nil, err
	}

	digest, err := layer.Digest()
	if err != nil {
		return nil, err
	}

	size, err := layer.Size()
	if err != nil {
		return nil, err
	}

	return &lifecycleLayer{
		Digest: digest.String(),
		DiffId: diffID.String(),
		Image:  imageRef,
		Size:   size,
	}, nil
}

func (l *LifecycleProvider) isLifecycleLoaded() bool {
	_, ok := l.lifecycleData.Load().(configmapRead)
	return ok
}

func (l *LifecycleProvider) lifecycle() (*lifecycle, error) {
	d, ok := l.lifecycleData.Load().(configmapRead)
	if !ok {
		return nil, errors.New("lifecycle image has not been loaded")
	}

	return d.lifecycle, d.err
}

func (l *LifecycleProvider) isNewImage(newLifecycle *lifecycle) bool {
	if !l.isLifecycleLoaded() {
		return true
	}

	lifecycle, err := l.lifecycle()
	if err != nil {
		return false
	}

	return lifecycle.digest.String() != newLifecycle.digest.String()
}

func (l *LifecycleProvider) callHandlers() {
	for _, cb := range l.handlers {
		cb()
	}
}

type configmapRead struct {
	lifecycle *lifecycle
	err       error
}

type lifecycle struct {
	digest   v1.Hash
	metadata cnb.LifecycleMetadata
	linux    *lifecycleLayer
	windows  *lifecycleLayer
	keychain authn.Keychain
}

type lifecycleLayer struct {
	Digest   string
	DiffId   string
	Image    string
	Size     int64
	Keychain authn.Keychain
}

func (l *lifecycleLayer) toLazyLayer(keychain authn.Keychain) (v1.Layer, error) {
	return imagehelpers.NewLazyMountableLayer(imagehelpers.LazyMountableLayerArgs{
		Digest:   l.Digest,
		DiffId:   l.DiffId,
		Image:    l.Image,
		Size:     l.Size,
		Keychain: keychain,
	})
}
