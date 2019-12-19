package registry

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
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
	reference, err := name.ParseReference(tag)
	if err != nil {
		return "", err
	}

	digest, err := image.Digest()
	if err != nil {
		return "", err
	}

	identifier := fmt.Sprintf("%s@%s", tag, digest.String())

	if digest.String() == previousDigest(reference, keychain, image) {
		return identifier, nil
	}

	return identifier, remote.Write(reference, image, remote.WithAuthFromKeychain(keychain))
}

func getIdentifier(image v1.Image, ref name.Reference) (string, error) {
	digest, err := image.Digest()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get digest for image '%s'", ref.Context().Name())
	}
	return fmt.Sprintf("%s@%s", ref.Context().Name(), digest), nil
}

func previousDigest(ref name.Reference, keychain authn.Keychain, passedIn v1.Image) string {

	pannedIn, _ := passedIn.RawManifest()

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(keychain))
	if err != nil {
		return ""
	}

	hash, err := img.Digest()
	if err != nil {
		return ""
	}
	obytes, _ := img.RawManifest()
	fmt.Println("expected")
	fmt.Println(string(pannedIn))
	fmt.Println("actual")
	fmt.Println(string(obytes))

	fmt.Println(cmp.Diff(string(pannedIn), string(obytes)))

	return hash.String()
}
