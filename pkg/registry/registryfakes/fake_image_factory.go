package registryfakes

import (
	"errors"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/pivotal/kpack/pkg/registry"
)

type fakeImageRecord struct {
	Image     *FakeRemoteImage
	SecretRef registry.SecretRef
}

type FakeImageFactory struct {
	images map[string]fakeImageRecord
}

func NewFakeImageFactory() *FakeImageFactory {
	return &FakeImageFactory{images: map[string]fakeImageRecord{}}
}

func (f *FakeImageFactory) AddImage(img *FakeRemoteImage, secret registry.SecretRef) error {
	ref, err := name.ParseReference(img.image, name.WeakValidation)
	if err != nil {
		return err
	}

	f.images[ref.String()] = fakeImageRecord{
		Image:     img,
		SecretRef: secret,
	}

	return nil
}

func (f *FakeImageFactory) NewRemote(image string, secretRef registry.SecretRef) (registry.RemoteImage, error) {
	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return nil, err
	}

	if record, ok := f.images[ref.String()]; ok {
		diff := cmp.Diff(record.SecretRef, secretRef)
		if diff == "" {
			return record.Image, nil
		}

		return nil, fmt.Errorf("invalid credentials for %s diff: %s", ref.String(), diff)
	}

	return nil, errors.New("image not found")
}
