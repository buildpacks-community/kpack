package registry

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "k8s.io/api/core/v1"
)

type SecretRef struct {
	ServiceAccount   string
	Namespace        string
	ImagePullSecrets []v1.LocalObjectReference
}

func (s SecretRef) IsNamespaced() bool {
	return s.Namespace != ""
}

func (s SecretRef) ServiceAccountOrDefault() string {
	if s.ServiceAccount == "" {
		return "default"
	}
	return s.ServiceAccount
}

type KeychainFactory interface {
	KeychainForSecretRef(context.Context, SecretRef) (authn.Keychain, error)
}
