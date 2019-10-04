package git

import (
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/memory"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const defaultRemote = "origin"

type Auth interface {
	Auth() transport.AuthMethod
}

type BasicAuth struct {
	Username string
	Password string
}


func (b BasicAuth) Auth() transport.AuthMethod {
	return &http.BasicAuth{
		Username: b.Username,
		Password: b.Password,
	}
}

type AnonymousAuth struct {
}

func (AnonymousAuth) Auth() transport.AuthMethod {
	return nil
}

type remoteGitResolver struct {
}

func (*remoteGitResolver) Resolve(auth Auth, sourceConfig v1alpha1.SourceConfig) (v1alpha1.ResolvedSourceConfig, error) {
	repo := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{sourceConfig.Git.URL},
	})
	references, err := repo.List(&git.ListOptions{
		Auth: auth.Auth(),
	})
	if err != nil {
		return v1alpha1.ResolvedSourceConfig{
			Git: &v1alpha1.ResolvedGitSource{
				URL:      sourceConfig.Git.URL,
				Revision: sourceConfig.Git.Revision, // maybe
				Type:     v1alpha1.Unknown,
				SubPath:  sourceConfig.SubPath,
			},
		}, nil
	}

	for _, ref := range references {
		if string(ref.Name().Short()) == sourceConfig.Git.Revision {
			return v1alpha1.ResolvedSourceConfig{
				Git: &v1alpha1.ResolvedGitSource{
					URL:      sourceConfig.Git.URL,
					Revision: ref.Hash().String(),
					Type:     sourceType(ref),
					SubPath:  sourceConfig.SubPath,
				},
			}, nil
		}
	}

	return v1alpha1.ResolvedSourceConfig{
		Git: &v1alpha1.ResolvedGitSource{
			URL:      sourceConfig.Git.URL,
			Revision: sourceConfig.Git.Revision,
			Type:     v1alpha1.Commit,
			SubPath:  sourceConfig.SubPath,
		},
	}, nil
}

func sourceType(reference *plumbing.Reference) v1alpha1.GitSourceKind {
	switch {
	case reference.Name().IsBranch():
		return v1alpha1.Branch
	case reference.Name().IsTag():
		return v1alpha1.Tag
	default:
		return v1alpha1.Unknown
	}
}
