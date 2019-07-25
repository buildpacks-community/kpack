package git

import (
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"gopkg.in/src-d/go-git.v4/storage/memory"

	"github.com/pivotal/build-service-beam/pkg/apis/build/v1alpha1"
)

type GitKeychain interface {
	Resolve(namespace, serviceAccount string, git v1alpha1.Git) (Auth, error)
}

type BasicAuth struct {
	Username string
	Password string
}

func (b BasicAuth) auth() transport.AuthMethod {
	return &http.BasicAuth{
		Username: b.Username,
		Password: b.Password,
	}
}

type AnonymousAuth struct {
}

func (AnonymousAuth) auth() transport.AuthMethod {
	return nil
}

type Auth interface {
	auth() transport.AuthMethod
}

type RemoteGitResolver struct {
}

const defaultRemote = "origin"

func (*RemoteGitResolver) Resolve(auth Auth, gitSource v1alpha1.Git) (v1alpha1.ResolvedGitSource, error) {
	repo := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{gitSource.URL},
	})
	references, err := repo.List(&git.ListOptions{
		Auth: auth.auth(),
	})
	if err != nil {
		return v1alpha1.ResolvedGitSource{
			URL:      gitSource.URL,
			Revision: gitSource.Revision, //maybe
			Type:     v1alpha1.Unknown,
		}, nil
	}

	for _, ref := range references {
		if string(ref.Name().Short()) == gitSource.Revision {
			return v1alpha1.ResolvedGitSource{
				URL:      gitSource.URL,
				Revision: ref.Hash().String(),
				Type:     sourceType(ref),
			}, nil
		}
	}

	return v1alpha1.ResolvedGitSource{
		URL:      gitSource.URL,
		Revision: gitSource.Revision,
		Type:     v1alpha1.Commit,
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
