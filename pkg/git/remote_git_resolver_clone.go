package git

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const commitSHARegex = `^[a-f0-9]{40}$`

var commitSHAValidator = regexp.MustCompile(commitSHARegex)

func (r *remoteGitResolver) ResolveByCloning(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	// git clone
	repository, err := gogit.Clone(memory.NewStorage(), nil, &gogit.CloneOptions{
		URL:           sourceConfig.Git.URL,
		Auth:          auth,
		RemoteName:    defaultRemote,
		ReferenceName: plumbing.ReferenceName(sourceConfig.Git.Revision),
		Depth:         1,
		Bare:          true,
		Filter:        packp.FilterBlobNone(),
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
