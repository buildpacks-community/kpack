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

	input := remoteGitCloneResolverInput{
		repository: repository,
		auth:       auth,
		revision:   sourceConfig.Git.Revision,
	}
	var output *remoteGitCloneResolverOutput
	errs := []error{}

	for _, resolver := range resolvers {
		output, err = resolver(input)
		if err != nil {
			errs = append(errs, err)
			fmt.Printf("resolver %T failed: %v\n", resolver, err)
			continue
		}
		if output != nil {
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
			Revision:             output.hash.String(),
			Tree:                 hashOfSubpath(sourceConfig.SubPath, output.hash, repository),
			Type:                 output.kind,
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

type remoteGitCloneResolverInput struct {
	repository *gogit.Repository
	auth       transport.AuthMethod
	revision   string
}
type remoteGitCloneResolverOutput struct {
	reference *plumbing.Reference
	hash      plumbing.Hash
	kind      corev1alpha1.GitSourceKind
}

type resolverFunc func(remoteGitCloneResolverInput) (*remoteGitCloneResolverOutput, error)

var resolvers = []resolverFunc{resolveBranch, resolveTag, resolveRevision, looksLikeACommit}

func resolveBranch(input remoteGitCloneResolverInput) (*remoteGitCloneResolverOutput, error) {
	branch := input.revision
	if !strings.HasPrefix(branch, "refs/heads/") {
		branch = "refs/heads/" + branch
	}
	err := input.repository.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemote,
		Auth:       input.auth,
		RefSpecs: []gogitconfig.RefSpec{
			gogitconfig.RefSpec(branch + ":" + branch),
		},
		Depth:  1,
		Filter: packp.FilterBlobNone(),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching: %w", err)
	}

	hash, err := input.repository.ResolveRevision(plumbing.Revision(branch))
	if err != nil {
		return nil, fmt.Errorf("resolving: %w", err)
	}

	resolvedRef, err := input.repository.Reference(plumbing.ReferenceName(branch), true)
	if err != nil {
		return nil, fmt.Errorf("referencing: %w", err)
	}

	return &remoteGitCloneResolverOutput{
		reference: resolvedRef,
		hash:      *hash,
		kind:      corev1alpha1.Branch,
	}, nil
}

func resolveTag(input remoteGitCloneResolverInput) (*remoteGitCloneResolverOutput, error) {
	tag := input.revision
	if !strings.HasPrefix(tag, "refs/tags/") {
		tag = "refs/tags/" + tag
	}
	err := input.repository.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemote,
		Auth:       input.auth,
		RefSpecs: []gogitconfig.RefSpec{
			gogitconfig.RefSpec(tag + ":" + tag),
		},
		Depth:  1,
		Filter: packp.FilterBlobNone(),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching: %w", err)
	}

	hash, err := input.repository.ResolveRevision(plumbing.Revision(tag))
	if err != nil {
		return nil, fmt.Errorf("resolving: %w", err)
	}

	resolvedTag, err := input.repository.Reference(plumbing.ReferenceName(tag), true)
	if err != nil {
		return nil, fmt.Errorf("referencing: %w", err)
	}

	return &remoteGitCloneResolverOutput{
		reference: resolvedTag,
		hash:      *hash,
		kind:      corev1alpha1.Tag,
	}, nil
}

func resolveRevision(input remoteGitCloneResolverInput) (*remoteGitCloneResolverOutput, error) {
	commitHash := input.revision
	h := plumbing.NewHash(commitHash)
	_, err := input.repository.Object(plumbing.AnyObject, h)
	if err != nil {
		return nil, fmt.Errorf("resolving revision: %w", err)
	}

	return &remoteGitCloneResolverOutput{
		reference: plumbing.NewHashReference(plumbing.ReferenceName(commitHash), h),
		hash:      h,
		kind:      corev1alpha1.Commit,
	}, nil
}

func looksLikeACommit(input remoteGitCloneResolverInput) (*remoteGitCloneResolverOutput, error) {
	revision := input.revision
	if !commitSHAValidator.MatchString(revision) {
		return nil, nil
	}

	hash := plumbing.NewHash(revision)

	return &remoteGitCloneResolverOutput{
		reference: plumbing.NewHashReference(plumbing.ReferenceName(revision), hash),
		hash:      hash,
		kind:      corev1alpha1.Commit,
	}, nil
}
