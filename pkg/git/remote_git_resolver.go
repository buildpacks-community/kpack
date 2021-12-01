package git

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	git2go "github.com/libgit2/git2go/v31"
	"github.com/pkg/errors"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const defaultRemote = "origin"

var discardLogger = log.New(ioutil.Discard, "", 0)

type remoteGitResolver struct {
}

func (*remoteGitResolver) Resolve(keychain GitKeychain, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	dir, err := ioutil.TempDir("", "git-resolve")
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, err
	}
	defer os.RemoveAll(dir)

	repository, err := git2go.InitRepository(dir, false)
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "initializing repo")
	}
	defer repository.Free()

	remote, err := repository.Remotes.CreateWithOptions(sourceConfig.Git.URL, &git2go.RemoteCreateOptions{
		Name:  defaultRemote,
		Flags: git2go.RemoteCreateSkipInsteadof,
	})
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "create remote")
	}
	defer remote.Free()

	proxyOptions, err := proxyFromEnv(sourceConfig.Git.URL)
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "getting proxy from env")
	}

	err = remote.ConnectFetch(&git2go.RemoteCallbacks{
		CredentialsCallback:      keychainAsCredentialsCallback(keychain),
		CertificateCheckCallback: certificateCheckCallback(discardLogger),
	}, &proxyOptions, nil)
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:      sourceConfig.Git.URL,
				Revision: sourceConfig.Git.Revision,
				Type:     corev1alpha1.Unknown,
				SubPath:  sourceConfig.SubPath,
			},
		}, nil
	}

	references, err := remote.Ls()
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "remote ls")
	}

	for _, ref := range references {
		for _, format := range refRevParseRules {
			if fmt.Sprintf(format, sourceConfig.Git.Revision) == ref.Name {
				return corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      sourceConfig.Git.URL,
						Revision: ref.Id.String(),
						Type:     sourceType(ref),
						SubPath:  sourceConfig.SubPath,
					},
				}, nil
			}
		}
	}

	return corev1alpha1.ResolvedSourceConfig{
		Git: &corev1alpha1.ResolvedGitSource{
			URL:      sourceConfig.Git.URL,
			Revision: sourceConfig.Git.Revision,
			Type:     corev1alpha1.Commit,
			SubPath:  sourceConfig.SubPath,
		},
	}, nil
}

func sourceType(reference git2go.RemoteHead) corev1alpha1.GitSourceKind {
	switch {
	case strings.HasPrefix(reference.Name, "refs/heads"):
		return corev1alpha1.Branch
	case strings.HasPrefix(reference.Name, "refs/tags"):
		return corev1alpha1.Tag
	default:
		return corev1alpha1.Unknown
	}
}

var refRevParseRules = []string{
	"refs/%s",
	"refs/tags/%s",
	"refs/heads/%s",
	"refs/remotes/%s",
	"refs/remotes/%s/HEAD",
}
