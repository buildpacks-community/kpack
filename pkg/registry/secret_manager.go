package registry

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/pivotal/build-service-system/pkg/secret"
)

type SecretManager struct {
	Client corev1.CoreV1Interface
}

const KnativeRegistryUrl = "build.knative.dev/docker-0"

func (m *SecretManager) secretForServiceAccountAndRegistry(serviceAccount, namespace string, reg name.Registry) (*secret.URLAndUser, error) {
	sa, err := m.Client.ServiceAccounts(namespace).Get(serviceAccount, meta_v1.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	secretForServiceAccount, err := m.secretForServiceAccount(sa, reg, namespace)
	if err != nil {
		return nil, err
	}

	if secretForServiceAccount == nil {
		return nil, errors.Errorf("credentials not found for: %s", reg)
	}
	registryUser := secret.NewURLAndUser(reg.RegistryStr(), string(secretForServiceAccount.Data["username"]), string(secretForServiceAccount.Data["password"]))
	return &registryUser, nil
}

func (m *SecretManager) secretForServiceAccount(account *v1.ServiceAccount, reg name.Registry, namespace string) (*v1.Secret, error) {
	for _, secretRef := range account.Secrets {
		secret, err := m.Client.Secrets(namespace).Get(secretRef.Name, meta_v1.GetOptions{})
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if Match(reg, secret.ObjectMeta.Annotations[KnativeRegistryUrl]) {
			return secret, nil
		}

	}
	return nil, nil
}
