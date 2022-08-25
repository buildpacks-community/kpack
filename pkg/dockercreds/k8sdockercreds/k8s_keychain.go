package k8sdockercreds

import (
	"context"
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "k8s.io/client-go/kubernetes"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds/azurecredentialhelperfix"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/secret"
)

var azureFileKeychain = azurecredentialhelperfix.AzureFileKeychain() // To support AZURE_CONTAINER_REGISTRY_CONFIG

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

func (f *k8sSecretKeychainFactory) KeychainForSecretRef(ctx context.Context, ref registry.SecretRef) (authn.Keychain, error) {
	if !ref.IsNamespaced() {
		k8sKeychain, err := k8schain.NewNoClient(context.Background())
		if err != nil {
			return nil, err
		}
		return authn.NewMultiKeychain(f.volumeKeychain, k8sKeychain, azureFileKeychain), nil // k8s keychain with no secrets
	}

	serviceAccountKeychain, err := keychainFromServiceAccount(ctx, ref, &secret.Fetcher{Client: f.client})
	if err != nil {
		return nil, err
	}

	k8sKeychain, err := k8schain.New(ctx, f.client, k8schain.Options{
		Namespace:          ref.Namespace,
		ServiceAccountName: ref.ServiceAccount,
		ImagePullSecrets:   toStringPullSecrets(ref.ImagePullSecrets),
	})
	if err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(serviceAccountKeychain, f.volumeKeychain, k8sKeychain, azureFileKeychain), nil
}

func toStringPullSecrets(secrets []corev1.LocalObjectReference) []string {
	var stringSecrets []string
	for _, s := range secrets {
		stringSecrets = append(stringSecrets, s.Name)
	}
	return stringSecrets
}

func keychainFromServiceAccount(ctx context.Context, secretRef registry.SecretRef, fetcher *secret.Fetcher) (authn.Keychain, error) {
	var dockerCreds dockercreds.DockerCreds

	secrets, err := fetcher.SecretsForServiceAccount(ctx, secretRef.ServiceAccountOrDefault(), secretRef.Namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		return dockerCreds, nil
	}

	for _, s := range secrets {
		switch s.Type {
		case corev1.SecretTypeBasicAuth:
			var err error
			if registry, ok := s.Annotations[buildapi.DOCKERSecretAnnotationPrefix]; ok {
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
