package cnb

import (
	"github.com/buildpack/imgutil"
	"github.com/buildpack/imgutil/remote"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/pivotal/kpack/pkg/registry"
)

type ImageFactory struct {
	KeychainFactory registry.KeychainFactory
}

//go:generate counterfeiter . RemoteImageUtilFactory
type RemoteImageUtilFactory interface {
	NewRemote(image string, secretRef registry.SecretRef) (imgutil.Image, error)
}

func (f *ImageFactory) NewRemote(image string, secretRef registry.SecretRef) (imgutil.Image, error) {
	keychain, err := f.KeychainFactory.KeychainForSecretRef(secretRef)
	if err != nil {
		return nil, err
	}
	imageRef, err := name.ParseReference(image)
	if err != nil {
		return nil, err
	}
	return remote.NewImage(imageRef.Context().Name(), keychain, remote.FromBaseImage(image))
}
