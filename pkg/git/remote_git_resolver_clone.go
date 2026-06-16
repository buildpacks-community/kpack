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

	input := fetchResolverInput{
		repository: repository,
		auth:       auth,
		revision:   sourceConfig.Git.Revision,
	}
	var output *fetchResolverOutput
	errs := []error{}

	for _, resolver := range relevantFetchResolversFor(sourceConfig.Git.Revision) {
		output, err = resolver(input)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if output != nil {
			errs = nil
			break
		}
	}

	err = errors.Join(errs...)
	if err != nil {
		return emptyResult, fmt.Errorf("revision \"%s\": unable to fetch references for repository: %w", sourceConfig.Git.Revision, err)
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

	// If the object is an annotated tag, first resolve to the commit it points
	// to.
	tag, err := repository.TagObject(hash)
	if err == nil {
		hash = tag.Target
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

type fetchResolverInput struct {
	repository *gogit.Repository
	auth       transport.AuthMethod
	revision   string
}
type fetchResolverOutput struct {
	reference *plumbing.Reference
	hash      plumbing.Hash
	kind      corev1alpha1.GitSourceKind
}

type fetchResolver func(fetchResolverInput) (*fetchResolverOutput, error)

var allFetchResolvers = []fetchResolver{resolveBranch, resolveTag, resolveRevision, looksLikeACommit}

func relevantFetchResolversFor(revision string) []fetchResolver {
	if strings.HasPrefix(revision, "refs/heads/") {
		return []fetchResolver{resolveBranch}
	}

	if strings.HasPrefix(revision, "refs/tags/") {
		return []fetchResolver{resolveTag}
	}

	if commitSHAValidator.MatchString(revision) {
		return []fetchResolver{resolveRevision, looksLikeACommit}
	}

	return allFetchResolvers
}

func resolveBranch(input fetchResolverInput) (*fetchResolverOutput, error) {
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

	return &fetchResolverOutput{
		reference: resolvedRef,
		hash:      *hash,
		kind:      corev1alpha1.Branch,
	}, nil
}

func resolveTag(input fetchResolverInput) (*fetchResolverOutput, error) {
	tag := input.revision
	ref := tag
	if !strings.HasPrefix(ref, "refs/tags/") {
		ref = "refs/tags/" + ref
	}
	err := input.repository.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemote,
		Auth:       input.auth,
		RefSpecs: []gogitconfig.RefSpec{
			gogitconfig.RefSpec(ref + ":" + ref),
		},
		Depth:  1,
		Filter: packp.FilterBlobNone(),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching: %w", err)
	}

	reference, err := input.repository.Tag(tag)
	if err != nil {
		return nil, fmt.Errorf("tag: %w", err)
	}

	return &fetchResolverOutput{
		reference: reference,
		hash:      reference.Hash(),
		kind:      corev1alpha1.Tag,
	}, nil
}

func resolveRevision(input fetchResolverInput) (*fetchResolverOutput, error) {
	commitHash := input.revision
	err := input.repository.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemote,
		Auth:       input.auth,
		RefSpecs: []gogitconfig.RefSpec{
			gogitconfig.RefSpec(commitHash + ":refs/heads/reference"),
		},
		Depth:  1,
		Filter: packp.FilterBlobNone(),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching: %w", err)
	}

	reference, err := input.repository.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return nil, fmt.Errorf("lookup: %w", err)
	}

	return &fetchResolverOutput{
		reference: plumbing.NewHashReference(plumbing.ReferenceName(commitHash), reference.Hash),
		hash:      reference.Hash,
		kind:      corev1alpha1.Commit,
	}, nil
}

func looksLikeACommit(input fetchResolverInput) (*fetchResolverOutput, error) {
	revision := input.revision
	if !commitSHAValidator.MatchString(revision) {
		return nil, nil
	}

	hash := plumbing.NewHash(revision)

	return &fetchResolverOutput{
		reference: plumbing.NewHashReference(plumbing.ReferenceName(revision), hash),
		hash:      hash,
		kind:      corev1alpha1.Commit,
	}, nil
}
