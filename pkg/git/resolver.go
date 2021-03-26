package git

import (
	"context"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type Resolver struct {
	remoteGitResolver remoteGitResolver
	gitKeychain       *k8sGitKeychain
}

func NewResolver(k8sClient k8sclient.Interface) *Resolver {
	return &Resolver{
		remoteGitResolver: remoteGitResolver{},
		gitKeychain:       newK8sGitKeychain(k8sClient),
	}
}

func (r *Resolver) Resolve(ctx context.Context, sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSourceConfig, error) {
	auth, err := r.gitKeychain.Resolve(ctx, sourceResolver.Namespace, sourceResolver.Spec.ServiceAccount, *sourceResolver.Spec.Source.Git)
	if err != nil {
		return v1alpha1.ResolvedSourceConfig{}, err
	}

	return r.remoteGitResolver.Resolve(auth, sourceResolver.Spec.Source)
}

func (*Resolver) CanResolve(sourceResolver *v1alpha1.SourceResolver) bool {
	return sourceResolver.IsGit()
}
