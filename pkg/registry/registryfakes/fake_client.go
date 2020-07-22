package registryfakes

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
)

func NewFakeClient() *FakeClient {
	return &FakeClient{
		images:         map[string]v1.Image{},
		readKeychains:  map[string]authn.Keychain{},
		savedImages:    map[string]v1.Image{},
		writeKeychains: map[string]authn.Keychain{},
	}
}

type FakeClient struct {
	images        map[string]v1.Image
	readKeychains map[string]authn.Keychain

	savedImages    map[string]v1.Image
	writeKeychains map[string]authn.Keychain
	fetchError     error
}

func (f *FakeClient) Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error) {
	if f.fetchError != nil {
		return nil, "", f.fetchError
	}

	if expectedKeychain, ok := f.readKeychains[repoName]; !ok || keychain != expectedKeychain {
		return nil, "", errors.New("unexpected keychain")
	}

	image, ok := f.images[repoName]
	if !ok {
		return nil, "", errors.Errorf("image %s not found in fake", repoName)
	}

	ref, err := name.ParseReference(repoName, name.WeakValidation)
	if err != nil {
		return nil, "", errors.Wrapf(err, "unable to parse %s", repoName)
	}

	digest, err := image.Digest()
	if err != nil {
		return nil, "", err
	}

	return image, fmt.Sprintf("%s@%s", ref.Context().Name(), digest), nil
}

func (f *FakeClient) Save(keychain authn.Keychain, tag string, image v1.Image) (string, error) {
	if expectedKeychain, ok := f.writeKeychains[tag]; !ok || keychain != expectedKeychain {
		return "", errors.New("unexpected keychain")
	}

	f.savedImages[tag] = image

	hash, err := image.Digest()
	return fmt.Sprintf("%s@%s", tag, hash), err
}

func (f *FakeClient) AddImage(repoName string, image v1.Image, keychain authn.Keychain) {
	f.images[repoName] = image
	f.readKeychains[repoName] = keychain
}

func (f *FakeClient) AddSaveKeychain(tag string, keychain authn.Keychain) {
	f.writeKeychains[tag] = keychain
}

func (f *FakeClient) SavedImages() map[string]v1.Image {
	return f.savedImages
}

func (f *FakeClient) SetFetchError(err error) {
	f.fetchError = err
}
