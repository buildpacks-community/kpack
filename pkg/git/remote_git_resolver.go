package git

import (
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	"github.com/go-git/go-git/v5/storage/memory"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const defaultRemote = "origin"

type remoteGitResolver struct{}

func (*remoteGitResolver) Resolve(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	remote := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{sourceConfig.Git.URL},
	})

	httpsTransport, err := getHttpsTransport()
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:      sourceConfig.Git.URL,
				Revision: sourceConfig.Git.Revision,
				Type:     corev1alpha1.Unknown,
				SubPath:  sourceConfig.SubPath,
			},
		}, nil
	}
	client.InstallProtocol("https", httpsTransport)

	refs, err := remote.List(&gogit.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:      sourceConfig.Git.URL,
				Revision: sourceConfig.Git.Revision,
				Type:     corev1alpha1.Unknown,
				SubPath:  sourceConfig.SubPath,
			},
		}, nil
	}

	for _, ref := range refs {
		if ref.Name().Short() == sourceConfig.Git.Revision {
			return corev1alpha1.ResolvedSourceConfig{
				Git: &corev1alpha1.ResolvedGitSource{
					URL:      sourceConfig.Git.URL,
					Revision: ref.Hash().String(),
					Type:     sourceType(ref),
					SubPath:  sourceConfig.SubPath,
				},
			}, nil
		}
	}

	return corev1alpha1.ResolvedSourceConfig{
		Git: &corev1alpha1.ResolvedGitSource{
			URL:      sourceConfig.Git.URL,
			Revision: sourceConfig.Git.Revision,
			Type:     corev1alpha1.Commit,
			SubPath:  sourceConfig.SubPath,
		},
	}, nil
}

func sourceType(reference *plumbing.Reference) corev1alpha1.GitSourceKind {
	switch {
	case reference.Name().IsBranch():
		return corev1alpha1.Branch
	case reference.Name().IsTag():
		return corev1alpha1.Tag
	default:
		return corev1alpha1.Unknown
	}
}
