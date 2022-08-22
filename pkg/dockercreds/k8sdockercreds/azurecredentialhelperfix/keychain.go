package azurecredentialhelperfix

import (
	"fmt"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	credentialprovider "github.com/vdemeester/k8s-pkg-credentialprovider"
)

var (
	once    sync.Once
	keyring credentialprovider.DockerKeyring
)

func AzureFileKeychain() authn.Keychain {
	once.Do(func() {
		keyring = credentialprovider.NewDockerKeyring()
	})

	return &keychain{keyring: keyring}
}

// Copied from https://github.com/google/go-containerregistry/blob/d9bfbcb99e526b2a9417160e209b816e1b1fb6bd/pkg/authn/k8schain/k8schain.go#L141
type lazyProvider struct {
	kc    *keychain
	image string
}

// Authorization implements Authenticator.
func (lp lazyProvider) Authorization() (*authn.AuthConfig, error) {
	creds, found := lp.kc.keyring.Lookup(lp.image)
	if !found || len(creds) < 1 {
		return nil, fmt.Errorf("keychain returned no credentials for %q", lp.image)
	}
	authConfig := creds[0]
	return &authn.AuthConfig{
		Username:      authConfig.Username,
		Password:      authConfig.Password,
		Auth:          authConfig.Auth,
		IdentityToken: authConfig.IdentityToken,
		RegistryToken: authConfig.RegistryToken,
	}, nil
}

type keychain struct {
	keyring credentialprovider.DockerKeyring
}

// Resolve implements authn.Keychain
func (kc *keychain) Resolve(target authn.Resource) (authn.Authenticator, error) {
	var image string
	if repo, ok := target.(name.Repository); ok {
		image = repo.String()
	} else {
		// Lookup expects an image reference and we only have a registry.
		image = target.RegistryStr() + "/foo/bar"
	}

	if creds, found := kc.keyring.Lookup(image); !found || len(creds) < 1 {
		return authn.Anonymous, nil
	}
	return lazyProvider{
		kc:    kc,
		image: image,
	}, nil
}
