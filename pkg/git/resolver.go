package git

import (
	"context"

	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
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

func (r *Resolver) Resolve(ctx context.Context, sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSourceConfig, error) {
	keychain, err := r.gitKeychain.KeychainForServiceAccount(ctx, sourceResolver.Namespace, sourceResolver.Spec.ServiceAccount)
	if err != nil {
		return v1alpha1.ResolvedSourceConfig{}, err
	}

	return r.remoteGitResolver.Resolve(keychain, sourceResolver.Spec.Source)
}

func (*Resolver) CanResolve(sourceResolver *v1alpha1.SourceResolver) bool {
	return sourceResolver.IsGit()
}
