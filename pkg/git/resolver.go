package git

import (
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	k8sclient "k8s.io/client-go/kubernetes"
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

func (r *Resolver) Resolve(sourceResolver *v1alpha2.SourceResolver) (v1alpha2.ResolvedSourceConfig, error) {
	auth, err := r.gitKeychain.Resolve(sourceResolver.Namespace, sourceResolver.Spec.ServiceAccount, *sourceResolver.Spec.Source.Git)
	if err != nil {
		return v1alpha2.ResolvedSourceConfig{}, err
	}

	return r.remoteGitResolver.Resolve(auth, sourceResolver.Spec.Source)
}

func (*Resolver) CanResolve(sourceResolver *v1alpha2.SourceResolver) bool {
	return sourceResolver.IsGit()
}
