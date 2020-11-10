package k8sdockercreds

import (
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	_ "github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds/azurecredentialhelperfix"

	"encoding/json"
	"sort"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/secret"
)

type k8sSecretKeychainFactory struct {
	client         k8sclient.Interface
	volumeKeychain authn.Keychain
}

func NewSecretKeychainFactory(client k8sclient.Interface) (*k8sSecretKeychainFactory, error) {
	volumeKeychain, err := dockercreds.NewVolumeSecretKeychain()
	if err != nil {
		return nil, err
	}

	return &k8sSecretKeychainFactory{client: client, volumeKeychain: volumeKeychain}, nil
}

func (f *k8sSecretKeychainFactory) KeychainForSecretRef(ref registry.SecretRef) (authn.Keychain, error) {
	if !ref.IsNamespaced() {
		keychain, err := k8schain.New(nil, k8schain.Options{})
		if err != nil {
			return nil, err
		}
		return authn.NewMultiKeychain(f.volumeKeychain, keychain), nil // k8s keychain with no secrets
	}

	secretFetcher := &secret.Fetcher{Client: f.client}

	basicAuthKeychain := &annotatedBasicAuthKeychain{
		secretRef:     ref,
		secretFetcher: secretFetcher,
	}

	dockerCfgKeychain := &dockerConfigKeychain{
		secretRef:     ref,
		secretFetcher: secretFetcher,
	}

	k8sKeychain, err := k8schain.New(f.client, k8schain.Options{
		Namespace:          ref.Namespace,
		ServiceAccountName: ref.ServiceAccount,
		ImagePullSecrets:   toStringPullSecrets(ref.ImagePullSecrets),
	})
	if err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(basicAuthKeychain, dockerCfgKeychain, f.volumeKeychain, k8sKeychain), nil
}

func toStringPullSecrets(secrets []corev1.LocalObjectReference) []string {
	var stringSecrets []string
	for _, s := range secrets {
		stringSecrets = append(stringSecrets, s.Name)
	}
	return stringSecrets
}

type annotatedBasicAuthKeychain struct {
	secretRef     registry.SecretRef
	secretFetcher *secret.Fetcher
}

func (k *annotatedBasicAuthKeychain) Resolve(res authn.Resource) (authn.Authenticator, error) {
	secrets, err := k.secretFetcher.SecretsForServiceAccount(k.secretRef.ServiceAccountOrDefault(), k.secretRef.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		return authn.Anonymous, nil
	}

	sort.Slice(secrets, func(i, j int) bool { return secrets[i].Name < secrets[j].Name })

	for _, s := range secrets {
		matcher := dockercreds.RegistryMatcher{Registry: s.Annotations[v1alpha2.DOCKERSecretAnnotationPrefix]}
		if matcher.Match(res.RegistryStr()) && s.Type == corev1.SecretTypeBasicAuth {

			return authn.FromConfig(authn.AuthConfig{
				Username: string(s.Data[corev1.BasicAuthUsernameKey]),
				Password: string(s.Data[corev1.BasicAuthPasswordKey]),
			}), nil
		}
	}
	return authn.Anonymous, nil
}

type dockerConfigKeychain struct {
	secretRef     registry.SecretRef
	secretFetcher *secret.Fetcher
}

type dockerConfigJson struct {
	Auths dockercreds.DockerCreds `json:"auths"`
}

func (d *dockerConfigKeychain) Resolve(res authn.Resource) (authn.Authenticator, error) {
	secrets, err := d.secretFetcher.SecretsForServiceAccount(d.secretRef.ServiceAccountOrDefault(), d.secretRef.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		return authn.Anonymous, nil
	}

	var dockerCreds dockercreds.DockerCreds

	for _, s := range secrets {
		switch s.Type {
		case corev1.SecretTypeDockerConfigJson:
			config := dockerConfigJson{
				Auths: map[string]authn.AuthConfig{},
			}

			err = json.Unmarshal(s.Data[corev1.DockerConfigJsonKey], &config)
			if err != nil {
				return nil, err
			}

			dockerCreds, err = dockerCreds.Append(config.Auths)
			if err != nil {
				return nil, err
			}
		case corev1.SecretTypeDockercfg:
			var cred dockercreds.DockerCreds

			err = json.Unmarshal(s.Data[corev1.DockerConfigKey], &cred)
			if err != nil {
				return nil, err
			}
			dockerCreds, err = dockerCreds.Append(cred)
			if err != nil {
				return nil, err
			}
		}
	}
	return dockerCreds.Resolve(res)
}
