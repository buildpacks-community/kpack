package git

import (
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
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

func (g *Resolver) Resolve(sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSource, error) {
	auth, err := g.gitKeychain.Resolve(sourceResolver.Namespace, sourceResolver.Spec.ServiceAccount, *sourceResolver.Spec.Source.Git)
	if err != nil {
		return nil, err
	}

	return g.remoteGitResolver.Resolve(auth, *sourceResolver.Spec.Source.Git)
}

func (g *Resolver) CanResolve(sourceResolver *v1alpha1.SourceResolver) bool {
	return sourceResolver.IsGit()
}
