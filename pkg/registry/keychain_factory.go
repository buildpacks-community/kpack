package registry

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "k8s.io/api/core/v1"
)

type ServiceAccountRef struct {
	ServiceAccount   string
	Namespace        string
	ImagePullSecrets []v1.LocalObjectReference
}

func (s ServiceAccountRef) IsNamespaced() bool {
	return s.Namespace != ""
}

func (s ServiceAccountRef) ServiceAccountOrDefault() string {
	if s.ServiceAccount == "" {
		return "default"
	}
	return s.ServiceAccount
}

type KeychainFactory interface {
	MultiKeychainFromServiceAccountRef(context.Context, ServiceAccountRef) (authn.Keychain, error)
}
