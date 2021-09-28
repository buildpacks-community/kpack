package git

import (
	"context"

	k8sclient "k8s.io/client-go/kubernetes"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type Resolver struct {
	remoteGitResolver remoteGitResolver
	gitKeychain       *k8sGitKeychainFactory
}

func NewResolver(k8sClient k8sclient.Interface) *Resolver {
	return &Resolver{
		remoteGitResolver: remoteGitResolver{},
		gitKeychain:       newK8sGitKeychainFactory(k8sClient),
	}
}

func (r *Resolver) Resolve(ctx context.Context, sourceResolver *buildapi.SourceResolver) (corev1alpha1.ResolvedSourceConfig, error) {
	keychain, err := r.gitKeychain.KeychainForServiceAccount(ctx, sourceResolver.Namespace, sourceResolver.Spec.ServiceAccountName)
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, err
	}

	return r.remoteGitResolver.Resolve(keychain, sourceResolver.Spec.Source)
}

func (*Resolver) CanResolve(sourceResolver *buildapi.SourceResolver) bool {
	return sourceResolver.IsGit()
}
