package registry

import (
	"encoding/base64"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

type SecretKeychainFactory struct {
	SecretManager *SecretManager
}

type serviceAccountKeychain struct {
	imageRef      ImageRef
	secretManager *SecretManager
}

func (k *serviceAccountKeychain) Resolve(reg name.Registry) (authn.Authenticator, error) {
	creds, err := k.secretManager.secretForServiceAccountAndRegistry(k.imageRef.ServiceAccount(), k.imageRef.Namespace(), reg)
	if err != nil {
		return nil, err
	}

	return auth(toBase64(fmt.Sprintf("%s:%s", creds.Username, creds.Password))), nil
}

type auth string

func (a auth) Authorization() (string, error) {
	return "Basic " + string(a), nil
}

func (f *SecretKeychainFactory) KeychainForImageRef(ref ImageRef) authn.Keychain {
	if ref.ServiceAccount() == "" {
		return &anonymousKeychain{}
	}

	return &serviceAccountKeychain{imageRef: ref, secretManager: f.SecretManager}
}

type anonymousKeychain struct {
}

func (anonymousKeychain) Resolve(name.Registry) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}

func toBase64(s string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(s)))
}
