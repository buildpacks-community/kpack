package registry

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
)

type Client struct {
}

func (t *Client) Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error) {
	reference, err := name.ParseReference(repoName)
	if err != nil {
		return nil, "", err
	}

	image, err := remote.Image(reference, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return nil, "", err
	}

	identifier, err := getIdentifier(image, reference)
	if err != nil {
		return nil, "", err
	}

	return image, identifier, nil
}

func (t *Client) Save(keychain authn.Keychain, tag string, image v1.Image) (string, error) {
	ref, err := name.ParseReference(tag)
	if err != nil {
		return "", err
	}

	digest, err := image.Digest()
	if err != nil {
		return "", err
	}

	identifier := fmt.Sprintf("%s@%s", tag, digest.String())

	if digest.String() == previousDigest(keychain, ref) {
		return identifier, nil
	}

	return identifier, remote.Write(ref, image, remote.WithAuthFromKeychain(keychain))
}

func getIdentifier(image v1.Image, ref name.Reference) (string, error) {
	digest, err := image.Digest()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get digest for image '%s'", ref.Context().Name())
	}
	return ref.Context().Name() + "@" + digest.String(), nil
}

func previousDigest(keychain authn.Keychain, ref name.Reference) string {
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return ""
	}

	hash, err := img.Digest()
	if err != nil {
		return ""
	}

	return hash.String()
}
