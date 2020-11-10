package git

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

const defaultRemote = "origin"

type remoteGitResolver struct {
}

func (*remoteGitResolver) Resolve(auth transport.AuthMethod, sourceConfig v1alpha2.SourceConfig) (v1alpha2.ResolvedSourceConfig, error) {
	repo := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{sourceConfig.Git.URL},
	})
	references, err := repo.List(&git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return v1alpha2.ResolvedSourceConfig{
			Git: &v1alpha2.ResolvedGitSource{
				URL:      sourceConfig.Git.URL,
				Revision: sourceConfig.Git.Revision, // maybe
				Type:     v1alpha2.Unknown,
				SubPath:  sourceConfig.SubPath,
			},
		}, nil
	}

	for _, ref := range references {
		if string(ref.Name().Short()) == sourceConfig.Git.Revision {
			return v1alpha2.ResolvedSourceConfig{
				Git: &v1alpha2.ResolvedGitSource{
					URL:      sourceConfig.Git.URL,
					Revision: ref.Hash().String(),
					Type:     sourceType(ref),
					SubPath:  sourceConfig.SubPath,
				},
			}, nil
		}
	}

	return v1alpha2.ResolvedSourceConfig{
		Git: &v1alpha2.ResolvedGitSource{
			URL:      sourceConfig.Git.URL,
			Revision: sourceConfig.Git.Revision,
			Type:     v1alpha2.Commit,
			SubPath:  sourceConfig.SubPath,
		},
	}, nil
}

func sourceType(reference *plumbing.Reference) v1alpha2.GitSourceKind {
	switch {
	case reference.Name().IsBranch():
		return v1alpha2.Branch
	case reference.Name().IsTag():
		return v1alpha2.Tag
	default:
		return v1alpha2.Unknown
	}
}
