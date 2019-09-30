package registry

import (
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
)

type ImageFactory struct {
	KeychainFactory KeychainFactory
}

func (f *ImageFactory) NewRemote(image string, secretRef SecretRef) (RemoteImage, error) {
	keychain, err := f.KeychainFactory.KeychainForSecretRef(secretRef)
	if err != nil {
		return nil, err
	}
	return NewGoContainerRegistryImage(image, keychain)
}

func (f *ImageFactory) NewRemoteWithDefaultKeychain(image string) (RemoteImage, error) {
	return NewGoContainerRegistryImage(image, authn.DefaultKeychain)
}

type KeychainFactory interface {
	KeychainForSecretRef(SecretRef) (authn.Keychain, error)
}

type RemoteImage interface {
	CreatedAt() (time.Time, error)
	Identifier() (string, error)
	Label(labelName string) (string, error)
	Env(key string) (string, error)
}

//go:generate counterfeiter . RemoteImageFactory
type RemoteImageFactory interface {
	NewRemote(image string, secretRef SecretRef) (RemoteImage, error)
	NewRemoteWithDefaultKeychain(image string) (RemoteImage, error)
}

type SecretRef struct {
	ServiceAccount   string
	Namespace        string
	ImagePullSecrets []string
}

func (s SecretRef) IsNamespaced() bool {
	return s.Namespace != ""
}

func (s SecretRef) ServiceAccountOrDefault() string {
	if s.ServiceAccount == "" {
		return "default"
	}
	return s.ServiceAccount
}
