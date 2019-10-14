package k8sdockercreds

import (
	// Note this line is separated to ensure it is loaded before any other import
	// This needs to happen because the init in the package should run before
	// the init of the go containerregistry
	_ "github.com/pivotal/kpack/pkg/dockercreds/k8svolume"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/secret"
)

type k8sSecretKeychainFactory struct {
	client k8sclient.Interface
}

func (f *k8sSecretKeychainFactory) KeychainForSecretRef(ref registry.SecretRef) (authn.Keychain, error) {
	if !ref.IsNamespaced() {
		return k8schain.New(nil, k8schain.Options{}) // k8s keychain with no secrets
	}

	annotatedBasicAuthKeychain := &annotatedBasicAuthKeychain{
		secretRef:     ref,
		secretManager: &secret.SecretManager{Client: f.client, AnnotationKey: v1alpha1.DOCKERSecretAnnotationPrefix, Matcher: dockercreds.RegistryMatch},
	}

	k8sKeychain, err := k8schain.New(f.client, k8schain.Options{
		Namespace:          ref.Namespace,
		ServiceAccountName: ref.ServiceAccount,
		ImagePullSecrets:   toStringPullSecrets(ref.ImagePullSecrets),
	})
	if err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(annotatedBasicAuthKeychain, k8sKeychain), nil
}

func toStringPullSecrets(secrets []v1.LocalObjectReference) []string {
	var stringSecrets []string
	for _, s := range secrets {
		stringSecrets = append(stringSecrets, s.Name)
	}
	return stringSecrets
}

func NewSecretKeychainFactory(client k8sclient.Interface) *k8sSecretKeychainFactory {
	return &k8sSecretKeychainFactory{
		client: client,
	}
}

type annotatedBasicAuthKeychain struct {
	secretRef     registry.SecretRef
	secretManager *secret.SecretManager
}

func (k *annotatedBasicAuthKeychain) Resolve(res authn.Resource) (authn.Authenticator, error) {
	creds, err := k.secretManager.SecretForServiceAccountAndURL(k.secretRef.ServiceAccountOrDefault(), k.secretRef.Namespace, res.RegistryStr())
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		return authn.Anonymous, nil
	}

	return &authn.Basic{Username: creds.Username, Password: creds.Password}, nil
}
