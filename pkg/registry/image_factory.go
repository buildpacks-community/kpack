package registry

import (
	"github.com/buildpack/imgutil"
	"github.com/pkg/errors"
)

type ImageFactory struct {
	KeychainFactory KeychainFactory
}

func (f *ImageFactory) NewRemote(imageRef ImageRef) (RemoteImage, error) {
	remote, err := imgutil.NewRemoteImage(imageRef.RepoName(), f.KeychainFactory.KeychainForImageRef(imageRef))
	return remote, errors.Wrapf(err, "could not create remote image from ref %s", imageRef.RepoName())
}
