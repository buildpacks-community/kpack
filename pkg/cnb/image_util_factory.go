package cnb

import (
	"github.com/buildpack/imgutil"
	"github.com/buildpack/imgutil/remote"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/registry"
)

type ImageFactory struct {
	KeychainFactory registry.KeychainFactory
}

type RemoteImageUtilFactory interface {
	newRemote(imageName string, baseImage string, secretRef registry.SecretRef) (imgutil.Image, error)
}

func (f *ImageFactory) newRemote(imageName string, baseImage string, secretRef registry.SecretRef) (imgutil.Image, error) {
	keychain, err := f.KeychainFactory.KeychainForSecretRef(secretRef)
	if err != nil {
		return nil, err
	}
	image, err := remote.NewImage(imageName, keychain, remote.FromBaseImage(baseImage))
	return image, errors.WithStack(err)
}
