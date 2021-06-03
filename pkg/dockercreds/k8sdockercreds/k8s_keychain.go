package k8sdockercreds

import (
	_ "github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds/azurecredentialhelperfix"

	"context"
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
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

func (f *k8sSecretKeychainFactory) MultiKeychainFromServiceAccountRef(ctx context.Context, ref registry.ServiceAccountRef) (authn.Keychain, error) {
	if !ref.IsNamespaced() {
		keychain, err := k8schain.New(ctx, nil, k8schain.Options{})
		if err != nil {
			return nil, err
		}
		return authn.NewMultiKeychain(f.volumeKeychain, keychain), nil // k8s keychain with no secrets
	}

	serviceAccountKeychain, err := f.KeychainFromServiceAccountSecrets(ctx, ref)
	if err != nil {
		return nil, err
	}

	k8sKeychain, err := f.keychainFromServiceAccountPullSecrets(ctx, ref)
	if err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(serviceAccountKeychain, f.volumeKeychain, k8sKeychain), nil
}

func (f *k8sSecretKeychainFactory) KeychainFromServiceAccountSecrets(ctx context.Context, ref registry.ServiceAccountRef) (authn.Keychain, error) {
	var dockerCreds dockercreds.DockerCreds

	fetcher := &secret.Fetcher{Client: f.client}
	secrets, err := fetcher.SecretsForServiceAccount(ctx, ref.ServiceAccountOrDefault(), ref.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		return dockerCreds, nil
	}

	for _, s := range secrets {
		switch s.Type {
		case corev1.SecretTypeBasicAuth:
			var err error
			if registry, ok := s.Annotations[v1alpha1.DOCKERSecretAnnotationPrefix]; ok {
				dockerCreds, err = dockerCreds.Append(dockercreds.DockerCreds{
					registry: authn.AuthConfig{
						Username: string(s.Data[corev1.BasicAuthUsernameKey]),
						Password: string(s.Data[corev1.BasicAuthPasswordKey]),
					},
				})
				if err != nil {
					return nil, err
				}
			}
		case corev1.SecretTypeDockerConfigJson:
			dockerConfig := struct {
				Auths dockercreds.DockerCreds `json:"auths"`
			}{}

			err := json.Unmarshal(s.Data[corev1.DockerConfigJsonKey], &dockerConfig)
			if err != nil {
				return nil, err
			}

			dockerCreds, err = dockerCreds.Append(dockerConfig.Auths)
			if err != nil {
				return nil, err
			}
		case corev1.SecretTypeDockercfg:
			var dockerCfg dockercreds.DockerCreds

			err := json.Unmarshal(s.Data[corev1.DockerConfigKey], &dockerCfg)
			if err != nil {
				return nil, err
			}
			dockerCreds, err = dockerCreds.Append(dockerCfg)
			if err != nil {
				return nil, err
			}
		}
	}
	return dockerCreds, nil
}

func (f *k8sSecretKeychainFactory) keychainFromServiceAccountPullSecrets(ctx context.Context, ref registry.ServiceAccountRef) (authn.Keychain, error) {
	return k8schain.New(ctx, f.client, k8schain.Options{
		Namespace:          ref.Namespace,
		ServiceAccountName: ref.ServiceAccount,
		ImagePullSecrets:   toStringPullSecrets(ref.ImagePullSecrets),
	})
}

func toStringPullSecrets(secrets []corev1.LocalObjectReference) []string {
	var stringSecrets []string
	for _, s := range secrets {
		stringSecrets = append(stringSecrets, s.Name)
	}
	return stringSecrets
}
