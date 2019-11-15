package registryfakes

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
)

func NewFakeClient() *FakeClient {
	return &FakeClient{
		images:      map[string]v1.Image{},
		savedImages: map[string]v1.Image{},
	}
}

type FakeClient struct {
	images           map[string]v1.Image
	expectedKeychain authn.Keychain

	savedImages map[string]v1.Image
}

func (f *FakeClient) Fetch(keychain authn.Keychain, repoName string) (v1.Image, error) {
	if keychain != f.expectedKeychain {
		return nil, errors.New("unexpected expectedKeychain")
	}

	image, ok := f.images[repoName]
	if !ok {
		return nil, errors.Errorf("image %s not found in fake", repoName)
	}

	return image, nil
}

func (f *FakeClient) Save(keychain authn.Keychain, tag string, image v1.Image) (string, error) {
	if keychain != f.expectedKeychain {
		return "", errors.New("unexpected expectedKeychain")
	}

	f.savedImages[tag] = image

	hash, err := image.Digest()
	return fmt.Sprintf("%s@%s", tag, hash), err
}

func (f *FakeClient) AddImage(repoName string, image v1.Image) {
	f.images[repoName] = image
}

func (f *FakeClient) ExpectedKeychain(keychain authn.Keychain) {
	f.expectedKeychain = keychain
}

func (f *FakeClient) SavedImages() map[string]v1.Image {
	return f.savedImages
}
