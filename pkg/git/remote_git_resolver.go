package git

import (
	"fmt"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"io/ioutil"
	"log"
	"os"

	"github.com/pkg/errors"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const defaultRemote = "origin"

var discardLogger = log.New(ioutil.Discard, "", 0)

type remoteGitResolver struct {
}

func (*remoteGitResolver) Resolve(keychain GitKeychain, sourceConfig corev1alpha1.SourceConfig) (corev1alpha1.ResolvedSourceConfig, error) {
	// initialize a new repository in a temporary directory
	dir, err := ioutil.TempDir("", "kpack-git")
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "creating temp dir")
	}
	defer os.RemoveAll(dir)
	// initialize a new repository
	repository, err := gogit.PlainInit(dir, false)
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "initializing repo")
	}
	// create a new remote
	remote, err := repository.CreateRemote(&config.RemoteConfig{
		Name: defaultRemote,
		URLs: []string{sourceConfig.Git.URL},
	})

	cred, err := keychain.Resolve(sourceConfig.Git.URL, "", CredentialTypeUserpass)
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "getting auth for url")
	}

	auth, err := cred.Cred()
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "getting auth for url")
	}

	// fetch the remote
	err = remote.Fetch(&gogit.FetchOptions{
		RemoteName: defaultRemote,
		Auth:       auth,
		Progress:   discardLogger.Writer(),
	})
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

	// get the remote references
	references, err := remote.List(&gogit.ListOptions{
		Auth: auth,
	})
	if err != nil {
		return corev1alpha1.ResolvedSourceConfig{}, errors.Wrap(err, "listing remote references")
	}

	// iterate over the references
	for _, reference := range references {
		// iterate over the revRefParseRules
		for _, revRefParseRule := range refRevParseRules {
			// return ResolvedSourceConfig if the sourceConfig.Git.Revision matches the reference.Name()
			if fmt.Sprintf(revRefParseRule, sourceConfig.Git.Revision) == reference.Name().String() {
				return corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      sourceConfig.Git.URL,
						Revision: reference.Hash().String(),
						Type:     referenceNameToType(reference.Name()),
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

type transportCallbackAdapter struct {
	keychain GitKeychain
}

func keychainAsAuth(keychain GitKeychain, url string, usernameFromUrl string, allowedTypes CredentialType) (transport.AuthMethod, error) {
	// Resolve(url string, usernameFromUrl string, allowedTypes CredentialType) (GoGitCredential, error)
	goGitCredential, err := keychain.Resolve(url, usernameFromUrl, allowedTypes)
	if err != nil {
		return nil, err
	}
	auth, err := goGitCredential.Cred()
	if err != nil {
		return nil, err
	}
	return auth, nil
}

func referenceNameToType(referenceName plumbing.ReferenceName) corev1alpha1.GitSourceKind {
	switch {
	case referenceName.IsBranch():
		return corev1alpha1.Branch
	case referenceName.IsTag():
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
