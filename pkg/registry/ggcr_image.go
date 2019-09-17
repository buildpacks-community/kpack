package registry

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/pkg/errors"
)

type GoContainerRegistryImage struct {
	image    v1.Image
	repoName string
}

func NewGoContainerRegistryImage(repoName string, keychain authn.Keychain) (*GoContainerRegistryImage, error) {
	image, err := newV1Image(keychain, repoName)
	if err != nil {
		return nil, err
	}

	ri := &GoContainerRegistryImage{
		repoName: repoName,
		image:    image,
	}

	return ri, nil
}

func newV1Image(keychain authn.Keychain, repoName string) (v1.Image, error) {
	var auth authn.Authenticator
	ref, err := name.ParseReference(repoName, name.WeakValidation)
	if err != nil {
		return nil, errors.Wrapf(err, "parse reference '%s'", repoName)
	}

	auth, err = keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return nil, errors.Wrapf(err, "resolving keychain for '%s'", ref.Context().Registry)
	}

	image, err := remote.Image(ref, remote.WithAuth(auth), remote.WithTransport(http.DefaultTransport))
	if err != nil {
		return nil, errors.Wrapf(err, "connect to registry store '%s'", repoName)
	}

	return image, nil
}

func (i *GoContainerRegistryImage) CreatedAt() (time.Time, error) {
	cfg, err := i.configFile()
	if err != nil {
		return time.Time{}, err
	}
	return cfg.Created.UTC(), nil
}

func (i *GoContainerRegistryImage) Env(key string) (string, error) {
	cfg, err := i.configFile()
	if err != nil {
		return "", err
	}
	for _, envVar := range cfg.Config.Env {
		parts := strings.Split(envVar, "=")
		if parts[0] == key {
			return parts[1], nil
		}
	}
	return "", nil
}

func (i *GoContainerRegistryImage) Label(key string) (string, error) {
	cfg, err := i.configFile()
	if err != nil {
		return "", err
	}
	labels := cfg.Config.Labels
	return labels[key], nil
}

func (i *GoContainerRegistryImage) Identifier() (string, error) {
	ref, err := name.ParseReference(i.repoName, name.WeakValidation)
	if err != nil {
		return "", err
	}

	digest, err := i.image.Digest()
	if err != nil {
		return "", errors.Wrapf(err, "failed to get digest for image '%s'", i.repoName)
	}

	return fmt.Sprintf("%s@%s", ref.Context().Name(), digest), nil
}

func (i *GoContainerRegistryImage) configFile() (*v1.ConfigFile, error) {
	cfg, err := i.image.ConfigFile()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get config for image '%s'", i.repoName)
	} else if cfg == nil {
		return nil, errors.Errorf("failed to get config for image '%s'", i.repoName)
	}

	return cfg, nil
}
