package secret

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/registry"
)

type SecretKeychainFactory struct {
	secretManager *SecretManager
}

func NewSecretKeychainFactory(client k8sclient.Interface) *SecretKeychainFactory {
	return &SecretKeychainFactory{
		secretManager: &SecretManager{
			Client:        client,
			AnnotationKey: v1alpha1.DOCKERSecretAnnotationPrefix,
			Matcher:       dockercreds.RegistryMatch,
		},
	}
}

type pullSecretKeychain struct {
	imageRef      registry.ImageRef
	secretManager *SecretManager
}

func (k *pullSecretKeychain) Resolve(registry name.Registry) (authn.Authenticator, error) {
	base64Auth, err := k.secretManager.SecretForImagePull(k.imageRef.Namespace(), k.imageRef.SecretName(), registry.RegistryStr())
	if err != nil {
		return nil, err
	}
	return dockercreds.Auth(base64Auth), nil
}

type serviceAccountKeychain struct {
	imageRef      registry.ImageRef
	secretManager *SecretManager
}

func (k *serviceAccountKeychain) Resolve(reg name.Registry) (authn.Authenticator, error) {
	creds, err := k.secretManager.SecretForServiceAccountAndURL(k.imageRef.ServiceAccount(), k.imageRef.Namespace(), reg.RegistryStr())
	if err != nil {
		return nil, err
	}

	return &authn.Basic{Username: creds.Username, Password: creds.Password}, nil
}

func (f *SecretKeychainFactory) KeychainForImageRef(ref registry.ImageRef) authn.Keychain {
	if !ref.HasSecret() {
		return &anonymousKeychain{}
	}
	if ref.ServiceAccount() == "" {
		return &pullSecretKeychain{imageRef: ref, secretManager: f.secretManager}
	}
	return &serviceAccountKeychain{imageRef: ref, secretManager: f.secretManager}
}

type anonymousKeychain struct {
}

func (anonymousKeychain) Resolve(name.Registry) (authn.Authenticator, error) {
	return authn.Anonymous, nil
}
