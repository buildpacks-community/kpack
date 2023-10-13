package secretfakes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

type FakeFetchSecret struct {
	FakeSecrets []*corev1.Secret
	ShouldError bool
	ErrorOut    error

	SecretsForServiceAccountFunc func(context.Context, string, string) ([]*corev1.Secret, error)
}

func (f *FakeFetchSecret) SecretsForServiceAccount(ctx context.Context, serviceAccount, namespace string) ([]*corev1.Secret, error) {
	if f.SecretsForServiceAccountFunc != nil {
		return f.SecretsForServiceAccount(ctx, serviceAccount, namespace)
	}

	if f.ShouldError {
		return nil, f.ErrorOut
	}

	return f.FakeSecrets, nil
}
