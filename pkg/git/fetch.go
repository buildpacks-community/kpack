package git

import (
	"github.com/BurntSushi/toml"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"log"
	"os"
	"path"

	// import RemoteConfig
	"github.com/go-git/go-git/v5/config"

	"github.com/pkg/errors"
)

type Fetcher struct {
	Logger   *log.Logger
	Keychain GitKeychain
}

func (f Fetcher) Fetch(dir, gitURL, gitRevision, metadataDir string) error {
	f.Logger.Printf("Cloning %q @ %q...", gitURL, gitRevision)

	// Initialize a repository in the directory using gogit.Init
	repository, err := gogit.PlainInit(dir, false)
	if err != nil {
		return errors.Wrap(err, "initializing repo")
	}

	remote, err := repository.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})
	if err != nil {
		return errors.Wrap(err, "creating remote")
	}

	err = remote.Fetch(&gogit.FetchOptions{
		RefSpecs: []config.RefSpec{
			config.RefSpec("refs/*:refs/*"),
		},
		Auth:       nil,
		Tags:       gogit.AllTags,
		RemoteName: defaultRemote,
	})
	if err != nil {
		return errors.Wrap(err, "fetching remote")
	}

	hash, err := resolveRevision(repository, gitRevision)
	if err != nil {
		return errors.Wrap(err, "resolving revision")
	}

	// Look up the commit using the hash
	commit, err := repository.CommitObject(*hash)
	if err != nil {
		return errors.Wrap(err, "looking up commit")
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return errors.Wrap(err, "getting worktree")
	}

	err = worktree.Checkout(&gogit.CheckoutOptions{})
	if err != nil {
		return errors.Wrap(err, "checking out blank")
	}

	err = worktree.Checkout(&gogit.CheckoutOptions{
		Hash:   plumbing.NewHash(hash.String()),
		Create: false,
	})

	// Write the git revision to the metadata directory
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
				Commit: commit.Hash.String(),
			},
		},
	}
	if err := toml.NewEncoder(projectMetadataFile).Encode(projectMd); err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for git repository: %s", metadataDir, gitRevision)
	}

	f.Logger.Printf("Successfully cloned %q @ %q in path %q", gitURL, gitRevision, dir)
	return nil
}

// Implement resolveRevision and return a plumbing.Hash and error
func resolveRevision(repository *gogit.Repository, gitRevision string) (*plumbing.Hash, error) {
	ref, err := repository.ResolveRevision(plumbing.Revision(gitRevision))
	if err != nil {
		return resolveCommit(gitRevision)
	}
	return ref, nil
}

func resolveCommit(gitRevision string) (*plumbing.Hash, error) {
	// Use plumbing.NewHash to create a new hash
	hash := plumbing.NewHash(gitRevision)
	// if hash is empty
	if hash == plumbing.ZeroHash {
		return nil, errors.Errorf("could not find reference: %s", gitRevision) //invalid hash
	}
	return &hash, nil
}

//func resolveRevision(repository *git2go.Repository, gitRevision string) (*git2go.Oid, error) {
//	ref, err := repository.References.Dwim(gitRevision)
//	if err != nil {
//		return resolveCommit(gitRevision)
//	}
//
//	return ref.Target(), nil
//}

//func resolveCommit(gitRevision string) (*git2go.Oid, error) {
//	oid, err := git2go.NewOid(gitRevision)
//	if err != nil {
//		return nil, errors.Errorf("could not find reference: %s", gitRevision) //invalid oid
//	}
//	return oid, nil
//}

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
