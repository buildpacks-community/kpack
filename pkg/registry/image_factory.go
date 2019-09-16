package registry

import (
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
)

type ImageFactory struct {
	KeychainFactory KeychainFactory
}

func (f *ImageFactory) NewRemote(imageRef ImageRef) (RemoteImage, error) {
	remoteImage, err := NewGoContainerRegistryImage(imageRef.Image(), f.KeychainFactory.KeychainForImageRef(imageRef))
	return remoteImage, err
}

type KeychainFactory interface {
	KeychainForImageRef(ImageRef) authn.Keychain
}

type ImageRef interface {
	ServiceAccount() string
	Namespace() string
	Image() string
	HasSecret() bool
	SecretName() string
}

type noAuthImageRef struct {
	identifier string
}

func (na *noAuthImageRef) SecretName() string {
	return ""
}

func NewNoAuthImageRef(identifier string) *noAuthImageRef {
	return &noAuthImageRef{identifier: identifier}
}

func (na *noAuthImageRef) Image() string {
	return na.identifier
}

func (noAuthImageRef) ServiceAccount() string {
	return ""
}

func (noAuthImageRef) HasSecret() bool {
	return false
}

func (noAuthImageRef) Namespace() string {
	return ""
}

type RemoteImage interface {
	CreatedAt() (time.Time, error)
	Identifier() (string, error)
	Label(labelName string) (string, error)
	Env(key string) (string, error)
}

//go:generate counterfeiter . RemoteImageFactory
type RemoteImageFactory interface {
	NewRemote(imageRef ImageRef) (RemoteImage, error)
}
