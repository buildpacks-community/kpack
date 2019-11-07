package secret

import (
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
)

type Fetcher struct {
	Client k8sclient.Interface
}

func (f *Fetcher) SecretsForServiceAccount(serviceAccount, namespace string) ([]*v1.Secret, error) {
	sa, err := f.Client.CoreV1().ServiceAccounts(namespace).Get(serviceAccount, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return f.secretsFromServiceAccount(sa, namespace)
}

func (f *Fetcher) secretsFromServiceAccount(account *v1.ServiceAccount, namespace string) ([]*v1.Secret, error) {
	var secrets []*v1.Secret
	for _, secretRef := range account.Secrets {
		secret, err := f.Client.CoreV1().Secrets(namespace).Get(secretRef.Name, meta_v1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}
