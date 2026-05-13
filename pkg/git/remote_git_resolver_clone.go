package git

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	gogitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/protocol/packp"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/memory"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const commitSHARegex = `^[a-f0-9]{40}$`

var commitSHAValidator = regexp.MustCompile(commitSHARegex)

func (r *remoteGitResolver) ResolveByCloning(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	emptyResult := corev1alpha1.ResolvedSourceConfig{
		Git: &corev1alpha1.ResolvedGitSource{
			URL:                  sourceConfig.Git.URL,
			Revision:             sourceConfig.Git.Revision,
			Type:                 corev1alpha1.Unknown,
			SubPath:              sourceConfig.SubPath,
			InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
		},
	}
	// git init --bare
	repository, err := gogit.Init(memory.NewStorage())
	if err != nil {
		return emptyResult, fmt.Errorf("initializing repository: %w", err)
	}

	_, err = repository.CreateRemote(&gogitconfig.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{sourceConfig.Git.URL},
	})
	if err != nil {
		return emptyResult, fmt.Errorf("creating remote: %w", err)
	}

	var resolvedRef *plumbing.Reference
	var hash plumbing.Hash
	var kind corev1alpha1.GitSourceKind
	errs := []error{}

	for _, resolver := range resolvers {
		resolvedRef, hash, kind, err = resolver(repository, auth, sourceConfig.Git.Revision)
		if err != nil {
			errs = append(errs, err)
			fmt.Printf("resolver %T failed: %v\n", resolver, err)
			continue
		}
		if resolvedRef != nil {
			errs = nil
			break
		}
	}
	err = errors.Join(errs...)

	// errs could contain errors from some resolvers, only one of them needs to be successful.
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:                  sourceConfig.Git.URL,
				Revision:             sourceConfig.Git.Revision,
				Type:                 corev1alpha1.Unknown,
				SubPath:              sourceConfig.SubPath,
				InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
			},
		}, fmt.Errorf("revision \"%s\": unable to fetch references for repository: %w", sourceConfig.Git.Revision, err)
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

type resolverFunc func(repository *gogit.Repository, auth transport.AuthMethod, revision string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error)

var resolvers = []resolverFunc{resolveBranch, resolveTag, resolveRevision, looksLikeACommit}

func resolveBranch(repository *gogit.Repository, auth transport.AuthMethod, branch string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	if !strings.HasPrefix(branch, "refs/heads/") {
		branch = "refs/heads/" + branch
	}
	fmt.Println("fetching branch", branch)
	err := repository.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemote,
		Auth:       auth,
		RefSpecs: []gogitconfig.RefSpec{
			gogitconfig.RefSpec(branch + ":" + branch),
		},
		Depth:  1,
		Filter: packp.FilterBlobNone(),
	})
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, fmt.Errorf("fetching: %w", err)
	}

	hash, err := repository.ResolveRevision(plumbing.Revision(branch))
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, fmt.Errorf("resolving: %w", err)
	}

	resolvedRef, err := repository.Reference(plumbing.ReferenceName(branch), true)
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, fmt.Errorf("referencing: %w", err)
	}

	return resolvedRef, *hash, corev1alpha1.Branch, nil
}

func resolveTag(repository *gogit.Repository, auth transport.AuthMethod, tag string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	if !strings.HasPrefix(tag, "refs/tags/") {
		tag = "refs/tags/" + tag
	}
	err := repository.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemote,
		Auth:       auth,
		RefSpecs: []gogitconfig.RefSpec{
			gogitconfig.RefSpec(tag + ":" + tag),
		},
		Depth:  1,
		Filter: packp.FilterBlobNone(),
	})
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, fmt.Errorf("fetching: %w", err)
	}

	hash, err := repository.ResolveRevision(plumbing.Revision(tag))
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, fmt.Errorf("resolving: %w", err)
	}

	resolvedTag, err := repository.Reference(plumbing.ReferenceName(tag), true)
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, fmt.Errorf("referencing: %w", err)
	}

	return resolvedTag, *hash, corev1alpha1.Tag, nil
}

func resolveRevision(repository *gogit.Repository, auth transport.AuthMethod, revision string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	h := plumbing.NewHash(revision)
	_, err := repository.Object(plumbing.AnyObject, h)
	if err != nil {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, err
	}

	return plumbing.NewHashReference(plumbing.ReferenceName(revision), h), h, corev1alpha1.Commit, nil
}

func looksLikeACommit(_ *gogit.Repository, auth transport.AuthMethod, revision string) (*plumbing.Reference, plumbing.Hash, corev1alpha1.GitSourceKind, error) {
	if !commitSHAValidator.MatchString(revision) {
		return nil, plumbing.Hash{}, corev1alpha1.Unknown, nil
	}

	hash := plumbing.NewHash(revision)

	return plumbing.NewHashReference(plumbing.ReferenceName(revision), hash), hash, corev1alpha1.Commit, nil
}
