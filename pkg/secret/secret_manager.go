package secret

import (
	"fmt"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclient "k8s.io/client-go/kubernetes"
)

type SecretManager struct {
	Client        k8sclient.Interface
	AnnotationKey string
	Matcher       Matcher
}

type Matcher func(url, annotatedUrl string) bool

func (m *SecretManager) SecretForServiceAccountAndURL(serviceAccount, namespace string, url string) (BasicAuth, error) {
	sa, err := m.Client.CoreV1().ServiceAccounts(namespace).Get(serviceAccount, meta_v1.GetOptions{})
	if err != nil {
		return BasicAuth{}, err
	}

	secret, err := m.secretForServiceAccount(sa, url, namespace)
	if err != nil {
		return BasicAuth{}, err
	}

	return BasicAuth{
		Username: string(secret.Data[v1.BasicAuthUsernameKey]),
		Password: string(secret.Data[v1.BasicAuthPasswordKey]),
	}, nil
}

func (m *SecretManager) secretForServiceAccount(account *v1.ServiceAccount, url string, namespace string) (*v1.Secret, error) {
	for _, secretRef := range account.Secrets {
		secret, err := m.Client.CoreV1().Secrets(namespace).Get(secretRef.Name, meta_v1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if m.Matcher(url, secret.Annotations[m.AnnotationKey]) && secret.Type == v1.SecretTypeBasicAuth {
			return secret, nil
		}

	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "Secret"}, fmt.Sprintf("secret for %s", url))
}
