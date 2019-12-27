package registryfakes

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
)

func NewFakeClient() *FakeClient {
	return &FakeClient{
		images:         map[string]v1.Image{},
		ids:            map[string]string{},
		readKeychains:  map[string]authn.Keychain{},
		savedImages:    map[string]v1.Image{},
		writeKeychains: map[string]authn.Keychain{},
	}
}

type FakeClient struct {
	images        map[string]v1.Image
	ids           map[string]string
	readKeychains map[string]authn.Keychain

	savedImages    map[string]v1.Image
	writeKeychains map[string]authn.Keychain
}

func (f *FakeClient) Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error) {
	if expectedKeychain, ok := f.readKeychains[repoName]; !ok || keychain != expectedKeychain {
		return nil, "", errors.New("unexpected keychain")
	}

	image, ok := f.images[repoName]
	if !ok {
		return nil, "", errors.Errorf("image %s not found in fake", repoName)
	}

	id, ok := f.ids[repoName]
	if !ok {
		return nil, "", errors.Errorf("image %s not found in fake", repoName)
	}

	return image, id, nil
}

func (f *FakeClient) Save(keychain authn.Keychain, tag string, image v1.Image) (string, error) {
	if expectedKeychain, ok := f.writeKeychains[tag]; !ok || keychain != expectedKeychain {
		return "", errors.New("unexpected keychain")
	}

	f.savedImages[tag] = image

	hash, err := image.Digest()
	return fmt.Sprintf("%s@%s", tag, hash), err
}

func (f *FakeClient) AddImage(repoName string, image v1.Image, id string, keychain authn.Keychain) {
	f.images[repoName] = image
	f.ids[repoName] = id
	f.readKeychains[repoName] = keychain
}

func (f *FakeClient) AddSaveKeychain(tag string, keychain authn.Keychain) {
	f.writeKeychains[tag] = keychain
}

func (f *FakeClient) SavedImages() map[string]v1.Image {
	return f.savedImages
}
