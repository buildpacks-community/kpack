package git

import (
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"log"
	"os"
	"path"

	"github.com/BurntSushi/toml"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/pkg/errors"
)

type Fetcher struct {
	Logger               *log.Logger
	Keychain             GitKeychain
	InitializeSubmodules bool
}

func init() {
	//remove multi_ack and multi_ack_detailed from unsupported capabilities to enable Azure DevOps git support
	transport.UnsupportedCapabilities = []capability.Capability{
		capability.ThinPack,
	}
}

func (f Fetcher) Fetch(dir, gitURL, gitRevision, metadataDir string) error {
	f.Logger.Printf("Cloning %q @ %q...", gitURL, gitRevision)
	auth, err := f.Keychain.Resolve(gitURL)
	if err != nil {
		return err
	}

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

	resolver := &remoteGitResolver{}
	resolvedSourceConfig, err := resolver.Resolve(auth, corev1alpha1.SourceConfig{
		Git: &corev1alpha1.Git{
			URL:                  gitURL,
			Revision:             gitRevision,
			InitializeSubmodules: f.InitializeSubmodules,
		},
		SubPath: "",
	})
	if err != nil {
		return errors.Wrap(err, "resolving source config")
	}

	err = remote.Fetch(&gogit.FetchOptions{
		RefSpecs: []config.RefSpec{config.RefSpec(resolvedSourceConfig.Git.Revision + ":" + resolvedSourceConfig.Git.Revision)},
		Auth:     auth,
		Depth:    1,
	})
	if err != nil && err != transport.ErrAuthenticationRequired {
		return errors.Wrapf(err, "unable to fetch references for repository")
	} else if err == transport.ErrAuthenticationRequired {
		return errors.Wrapf(err, "invalid credentials for repository")
	}

	worktree, err := repository.Worktree()
	if err != nil {
		return errors.Wrapf(err, "getting worktree for repository")
	}

	//resolvedSourceConfig.Git.Revision is the hash of the commit
	hash := plumbing.NewHash(resolvedSourceConfig.Git.Revision)

	err = worktree.Checkout(&gogit.CheckoutOptions{Hash: hash})
	if err != nil {
		return errors.Wrapf(err, "checking out revision")
	}

	if f.InitializeSubmodules {
		submodules, err := worktree.Submodules()
		if err != nil {
			return errors.Wrapf(err, "getting submodules")
		}

		for _, submodule := range submodules {
			f.Logger.Printf("Updating submodule %v", submodule.Config().URL)
			submoduleAuth, err := f.Keychain.Resolve(submodule.Config().URL)
			if err != nil {
				return err
			}
			err = submodule.Update(&gogit.SubmoduleUpdateOptions{Auth: submoduleAuth, Init: true, RecurseSubmodules: gogit.DefaultSubmoduleRecursionDepth})
			if err != nil {
				return errors.Wrapf(err, "updating submodules")
			}
		}
	}

	projectMetadataFile, err := os.Create(path.Join(metadataDir, "project-metadata.toml"))
	if err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for git repository: %s", metadataDir, gitURL)
	}
	defer projectMetadataFile.Close()

	projectMd := Project{
		Source: Source{
			Type: "git",
			Metadata: Metadata{
				Repository: gitURL,
				Revision:   gitRevision,
			},
			Version: Version{
				Commit: hash.String(),
			},
		},
	}
	if err := toml.NewEncoder(projectMetadataFile).Encode(projectMd); err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for git repository: %s", metadataDir, gitRevision)
	}

	f.Logger.Printf("Successfully cloned %q @ %q in path %q", gitURL, gitRevision, dir)
	return nil
}

type Project struct {
	Source Source `toml:"source"`
}

type Source struct {
	Type     string   `toml:"type"`
	Metadata Metadata `toml:"metadata"`
	Version  Version  `toml:"version"`
}

type Metadata struct {
	Repository string `toml:"repository"`
	Revision   string `toml:"revision"`
}

type Version struct {
	Commit string `toml:"commit"`
}
