package git

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	gogitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/config"
)

const defaultRemote = "origin"
const commitSHARegex = `^[a-f0-9]{40}$`

var commitSHAValidator = regexp.MustCompile(commitSHARegex)

type remoteGitResolver struct {
	featureFlags config.FeatureFlags
}

func (r *remoteGitResolver) Resolve(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	if r.featureFlags.GitResolverUseShallowClone {
		return r.ResolveByCloning(auth, sourceConfig)
	}

	return r.ResolveByListingRemote(auth, sourceConfig)
}

func (r *remoteGitResolver) ResolveByListingRemote(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	remote := gogit.NewRemote(memory.NewStorage(), &gogitconfig.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{sourceConfig.Git.URL},
	})

	refs, err := remote.List(&gogit.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:                  sourceConfig.Git.URL,
				Revision:             sourceConfig.Git.Revision,
				Type:                 corev1alpha1.Unknown,
				SubPath:              sourceConfig.SubPath,
				InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
			},
		}, nil
	}

	for _, ref := range refs {
		if ref.Name().Short() == sourceConfig.Git.Revision {
			return corev1alpha1.ResolvedSourceConfig{
				Git: &corev1alpha1.ResolvedGitSource{
					URL:                  sourceConfig.Git.URL,
					Revision:             ref.Hash().String(),
					Type:                 sourceType(ref),
					SubPath:              sourceConfig.SubPath,
					InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
				},
			}, nil
		}
	}

	return corev1alpha1.ResolvedSourceConfig{
		Git: &corev1alpha1.ResolvedGitSource{
			URL:                  sourceConfig.Git.URL,
			Revision:             sourceConfig.Git.Revision,
			Type:                 corev1alpha1.Commit,
			SubPath:              sourceConfig.SubPath,
			InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
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

func (r *remoteGitResolver) ResolveByCloning(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	// git clone
	repository, err := gogit.Clone(memory.NewStorage(), nil, &gogit.CloneOptions{
		URL:           sourceConfig.Git.URL,
		Auth:          auth,
		RemoteName:    defaultRemote,
		ReferenceName: plumbing.ReferenceName(sourceConfig.Git.Revision),
		Depth:         1,
	})

	var resolvedRef *plumbing.Reference
	var hash plumbing.Hash
	var kind corev1alpha1.GitSourceKind
	errs := []error{}

	for _, resolver := range resolvers {
		resolvedRef, hash, kind, err = resolver(repository, sourceConfig.Git.Revision)
		if err != nil {
			errs = append(errs, err)
		}
		if resolvedRef != nil {
			break
		}
	}

	if resolvedRef == nil {
		return corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:                  sourceConfig.Git.URL,
				Revision:             sourceConfig.Git.Revision,
				Type:                 corev1alpha1.Unknown,
				SubPath:              sourceConfig.SubPath,
				InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
			},
		}, fmt.Errorf("revision \"%s\": unable to fetch references for repository: %w", sourceConfig.Git.Revision, errors.Join(errs...))
	}

	return corev1alpha1.ResolvedSourceConfig{
		Git: &corev1alpha1.ResolvedGitSource{
			URL:                  sourceConfig.Git.URL,
			Revision:             hash.String(),
			Tree:                 hashOfSubpath(sourceConfig.SubPath, hash, repository),
			Type:                 kind,
			SubPath:              sourceConfig.SubPath,
			InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
		},
	}, nil
}

func hashOfSubpath(subPath string, hash plumbing.Hash, repository *gogit.Repository) string {
	path := strings.Trim(subPath, "/")
	if path == "" {
		return ""
	}

	commit, err := repository.CommitObject(hash)
	if err != nil {
		return ""
	}

	treeObject, err := commit.Tree()
	if err != nil {
		return ""
	}

	entry, err := treeObject.FindEntry(path)
	if err != nil {
		return ""
	}

	return entry.Hash.String()

}

type resolverFunc func(repository *gogit.Repository, revision string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error)

var resolvers = []resolverFunc{resolveBranch, resolveTag, resolveRevision, looksLikeACommit}

func resolveBranch(repository *gogit.Repository, branch string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	resolvedBranch, err := repository.Branch(branch)
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, err
	}

	resolvedRef := plumbing.NewSymbolicReference(plumbing.ReferenceName(branch), plumbing.ReferenceName(resolvedBranch.Merge))
	h, err := repository.ResolveRevision(plumbing.Revision(resolvedBranch.Merge))
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, err
	}

	return resolvedRef, *h, corev1alpha1.Branch, nil
}

func resolveTag(repository *gogit.Repository, tag string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	resolvedTag, err := repository.Tag(tag)
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, err
	}

	return resolvedTag, resolvedTag.Hash(), corev1alpha1.Tag, nil
}

func resolveRevision(repository *gogit.Repository, revision string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	h := plumbing.NewHash(revision)
	_, err := repository.Object(plumbing.AnyObject, h)
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, err
	}

	return plumbing.NewHashReference(plumbing.ReferenceName(revision), h), h, corev1alpha1.Commit, nil
}

func looksLikeACommit(_ *gogit.Repository, revision string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	if !commitSHAValidator.MatchString(revision) {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, nil
	}

	hash := plumbing.NewHash(revision)

	return plumbing.NewHashReference(plumbing.ReferenceName(revision), hash), hash, corev1alpha1.Commit, nil
}
