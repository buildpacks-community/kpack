package registry

import (
	"encoding/base64"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/secret"
)

type SecretKeychainFactory struct {
	secretManager *secret.SecretManager
}

func NewSecretKeychainFactory(client k8sclient.Interface) *SecretKeychainFactory {
	return &SecretKeychainFactory{
		secretManager: &secret.SecretManager{
			Client:        client,
			AnnotationKey: v1alpha1.DOCKERSecretAnnotationPrefix,
			Matcher:       registryMatcher{},
		},
	}
}

type serviceAccountKeychain struct {
	imageRef      ImageRef
	secretManager *secret.SecretManager
}

func (k *serviceAccountKeychain) Resolve(reg name.Registry) (authn.Authenticator, error) {
	creds, err := k.secretManager.SecretForServiceAccountAndURL(k.imageRef.ServiceAccount(), k.imageRef.Namespace(), reg.RegistryStr())
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

	return &serviceAccountKeychain{imageRef: ref, secretManager: f.secretManager}
}

type anonymousKeychain struct {
}

func (anonymousKeychain) Resolve(name.Registry) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}

func toBase64(s string) []byte {
	return []byte(base64.StdEncoding.EncodeToString([]byte(s)))
}
