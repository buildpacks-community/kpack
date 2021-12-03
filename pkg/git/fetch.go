package git

import (
	"log"
	"os"
	"path"

	"github.com/BurntSushi/toml"
	git2go "github.com/libgit2/git2go/v31"
	"github.com/pkg/errors"
)

type Fetcher struct {
	Logger   *log.Logger
	Keychain GitKeychain
}

func (f Fetcher) Fetch(dir, gitURL, gitRevision, metadataDir string) error {
	f.Logger.Printf("Cloning %q @ %q...", gitURL, gitRevision)

	repository, err := git2go.InitRepository(dir, false)
	if err != nil {
		return errors.Wrap(err, "initializing repo")
	}
	defer repository.Free()

	remote, err := repository.Remotes.CreateWithOptions(gitURL, &git2go.RemoteCreateOptions{
		Name:  "origin",
		Flags: git2go.RemoteCreateSkipInsteadof,
	})
	if err != nil {
		return errors.Wrap(err, "creating remote")
	}
	defer remote.Free()

	proxyOptions, err := proxyFromEnv(gitURL)
	if err != nil {
		return errors.Wrap(err, "getting proxy from env")
	}

	err = remote.Fetch([]string{"refs/*:refs/*"}, &git2go.FetchOptions{
		DownloadTags: git2go.DownloadTagsAll,
		RemoteCallbacks: git2go.RemoteCallbacks{
			CredentialsCallback:      keychainAsCredentialsCallback(f.Keychain),
			CertificateCheckCallback: certificateCheckCallback(f.Logger),
		},
		ProxyOptions: proxyOptions,
	}, "")
	if err != nil {
		return errors.Wrap(err, "fetching remote")
	}

	oid, err := resolveRevision(repository, gitRevision)
	if err != nil {
		return err
	}

	commit, err := repository.LookupCommit(oid)
	if err != nil {
		return errors.Wrap(err, "looking up commit")
	}

	err = repository.SetHeadDetached(commit.Id())
	if err != nil {
		return errors.Wrap(err, "setting head detached")
	}
	err = repository.CheckoutHead(&git2go.CheckoutOpts{
		Strategy: git2go.CheckoutForce,
	})
	if err != nil {
		return errors.Wrap(err, "checkout head")
	}

	projectMetadataFile, err := os.Create(path.Join(metadataDir, "project-metadata.toml"))
	if err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for git repository: %s", metadataDir, gitURL)
	}
	defer projectMetadataFile.Close()

	projectMd := project{
		Source: source{
			Type: "git",
			Metadata: metadata{
				Repository: gitURL,
				Revision:   gitRevision,
			},
			Version: version{
				Commit: commit.Id().String(),
			},
		},
	}
	if err := toml.NewEncoder(projectMetadataFile).Encode(projectMd); err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for git repository: %s", metadataDir, gitRevision)
	}

	f.Logger.Printf("Successfully cloned %q @ %q in path %q", gitURL, gitRevision, dir)
	return nil
}

func resolveRevision(repository *git2go.Repository, gitRevision string) (*git2go.Oid, error) {
	ref, err := repository.References.Dwim(gitRevision)
	if err != nil {
		return resolveCommit(gitRevision)
	}

	return ref.Target(), nil
}

func resolveCommit(gitRevision string) (*git2go.Oid, error) {
	oid, err := git2go.NewOid(gitRevision)
	if err != nil {
		return nil, errors.Errorf("could not find reference: %s", gitRevision) //invalid oid
	}
	return oid, nil
}

type project struct {
	Source source `toml:"source"`
}

type source struct {
	Type     string   `toml:"type"`
	Metadata metadata `toml:"metadata"`
	Version  version  `toml:"version"`
}

type metadata struct {
	Repository string `toml:"repository"`
	Revision   string `toml:"revision"`
}

type version struct {
	Commit string `toml:"commit"`
}
