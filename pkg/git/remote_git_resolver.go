package git

import (
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"regexp"
	"strings"
)

type remoteGitResolver struct{}

func (*remoteGitResolver) Resolve(auth transport.AuthMethod, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	var resolvedConfig corev1alpha1.ResolvedSourceConfig

	r, err := gogit.Clone(memory.NewStorage(), nil, &gogit.CloneOptions{
		URL:  sourceConfig.Git.URL,
		Auth: auth,
	})

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

	rIter, _ := r.References()

	rIter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Name().Short() == sourceConfig.Git.Revision {
			refSourceType := sourceType(ref)
			effectiveCommit := ""

			if refSourceType == corev1alpha1.Branch {
				effectiveCommit, err = getLogs(r, sourceConfig.SubPath)
			} else {
				effectiveCommit = ref.Hash().String()
			}

			if err != nil {
				return nil
			}

			resolvedConfig = corev1alpha1.ResolvedSourceConfig{
				Git: &corev1alpha1.ResolvedGitSource{
					URL:      sourceConfig.Git.URL,
					Revision: effectiveCommit,
					Type:     refSourceType,
					SubPath:  sourceConfig.SubPath,
				},
			}
		}
		return nil
	})

	if resolvedConfig.ResolvedSource() == nil {
		resolvedConfig = corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:      sourceConfig.Git.URL,
				Revision: sourceConfig.Git.Revision,
				Type:     corev1alpha1.Commit,
				SubPath:  sourceConfig.SubPath,
			},
		}
	}

	return resolvedConfig, err
}

func getLogs(r *gogit.Repository, subPath string) (string, error) {
	logOutput, err := r.Log(&gogit.LogOptions{
		PathFilter: func(s string) bool {
			if strings.Contains(s, subPath) {
				return true
			} else {
				return false
			}
		},
	})

	latestCommit, err := logOutput.Next()
	regex, _ := regexp.Compile("[a-f0-9]{40}")
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
