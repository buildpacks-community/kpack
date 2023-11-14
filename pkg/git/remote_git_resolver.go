package git

import (
	"regexp"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const defaultRemote = "origin"

var regex = regexp.MustCompile("[a-f0-9]{40}")

type remoteGitResolver struct{}

func (*remoteGitResolver) Resolve(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	var resolvedConfig corev1alpha1.ResolvedSourceConfig
	var err error

	if sourceConfig.SubPath != "" {
		resolvedConfig, err = resolveSourceWithSubpath(auth, sourceConfig)
	} else {
		resolvedConfig, err = resolveSourceWithoutSubpath(auth, sourceConfig)
	}

	if err != nil {
		resolvedConfig = corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:                  sourceConfig.Git.URL,
				Revision:             sourceConfig.Git.Revision,
				Type:                 corev1alpha1.Unknown,
				SubPath:              sourceConfig.SubPath,
				InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
			},
		}
	}

	if resolvedConfig.ResolvedSource() == nil {
		resolvedConfig = corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:                  sourceConfig.Git.URL,
				Revision:             sourceConfig.Git.Revision,
				Type:                 corev1alpha1.Commit,
				SubPath:              sourceConfig.SubPath,
				InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
			},
		}
	}

	return resolvedConfig, err
}

func resolveSourceWithoutSubpath(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	remote := gogit.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{sourceConfig.Git.URL},
	})

	refs, err := remote.List(&gogit.ListOptions{
		Auth: auth,
	})

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
	return corev1alpha1.ResolvedSourceConfig{}, err
}

func resolveSourceWithSubpath(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	var resolvedConfig corev1alpha1.ResolvedSourceConfig

	r, err := gogit.Clone(memory.NewStorage(), nil, &gogit.CloneOptions{
		URL:  sourceConfig.Git.URL,
		Auth: auth,
	})

	rIter, _ := r.References()

	rIter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().Short() == sourceConfig.Git.Revision {
			refSourceType := sourceType(ref)
			effectiveCommit := ""

			if refSourceType == corev1alpha1.Branch {
				effectiveCommit, err = latestCommitForSubpath(r, sourceConfig.SubPath)
			} else {
				effectiveCommit = ref.Hash().String()
			}

			if err != nil {
				return nil
			}

			resolvedConfig = corev1alpha1.ResolvedSourceConfig{
				Git: &corev1alpha1.ResolvedGitSource{
					URL:                  sourceConfig.Git.URL,
					Revision:             effectiveCommit,
					Type:                 refSourceType,
					SubPath:              sourceConfig.SubPath,
					InitializeSubmodules: sourceConfig.Git.InitializeSubmodules,
				},
			}
		}
		return nil
	})

	return resolvedConfig, err
}

func latestCommitForSubpath(r *gogit.Repository, subPath string) (string, error) {
	logOutput, err := r.Log(&gogit.LogOptions{
		PathFilter: func(s string) bool {
			if strings.HasPrefix(s, subPath) {
				return true
			} else {
				return false
			}
		},
	})

	latestCommit, err := logOutput.Next()
	effectiveCommit := regex.FindString(latestCommit.String())

	return effectiveCommit, err
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
