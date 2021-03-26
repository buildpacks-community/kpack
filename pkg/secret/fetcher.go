package secret

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
)

type Fetcher struct {
	Client k8sclient.Interface
}

func (f *Fetcher) SecretsForServiceAccount(ctx context.Context, serviceAccount, namespace string) ([]*corev1.Secret, error) {
	sa, err := f.Client.CoreV1().ServiceAccounts(namespace).Get(ctx, serviceAccount, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return f.secretsFromServiceAccount(ctx, sa, namespace)
}

func (f *Fetcher) secretsFromServiceAccount(ctx context.Context, account *corev1.ServiceAccount, namespace string) ([]*corev1.Secret, error) {
	var secrets []*corev1.Secret
	for _, secretRef := range account.Secrets {
		secret, err := f.Client.CoreV1().Secrets(namespace).Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, secret)
	}
	return secrets, nil
}
