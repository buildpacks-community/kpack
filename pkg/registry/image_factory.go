package registry

import (
	"github.com/buildpack/lifecycle/image"
	"github.com/pkg/errors"
)

type ImageFactory struct {
	KeychainFactory KeychainFactory
}

func (f *ImageFactory) NewRemote(imageRef ImageRef) (RemoteImage, error) {
	factory, err := image.NewFactory()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	factory.Keychain = f.KeychainFactory.KeychainForImageRef(imageRef)

	remote, err := factory.NewRemote(imageRef.RepoName())
	return remote, errors.Wrapf(err, "could not create remote image from ref %s", imageRef.RepoName())
}
