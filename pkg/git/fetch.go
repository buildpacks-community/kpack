package git

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/BurntSushi/toml"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/pkg/errors"
)

type Fetcher struct {
	Logger   *log.Logger
	Keychain GitKeychain
}

func (f Fetcher) Fetch(dir, gitURL, gitRevision, metadataDir string) error {
	resolvedAuth, err := f.Keychain.Resolve(gitURL)
	if err != nil {
		return err
	}

	tmpDir, err := ioutil.TempDir("", "git-clone-")
	if err != nil {
		return err
	}

	repo, err := git.PlainInit(tmpDir, false)
	if err != nil {
		return errors.Wrap(err, "unable to init git repository")
	}

	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	})
	if err != nil {
		return errors.Wrap(err, "unable to create remote")
	}

	opts := &git.FetchOptions{
		RefSpecs: []config.RefSpec{"refs/*:refs/*"},
		Auth:     resolvedAuth,
		Depth:    0,
	}

	f.Logger.Printf("Cloning %q @ %q...", gitURL, gitRevision)
	err = remote.Fetch(opts)
	if err != nil && err != transport.ErrAuthenticationRequired {
		return errors.Wrap(err, "unable to fetch git repository")
	} else if err == transport.ErrAuthenticationRequired {
		return errors.Errorf("invalid credentials to fetch git repository: %s", gitURL)
	}

	workTree, err := repo.Worktree()
	if err != nil {
		return errors.Wrap(err, "unable to retrieve working tree")
	}

	hashes, err := repo.ResolveRevision(plumbing.Revision(gitRevision))
	if err != nil {
		return errors.Wrapf(err, "resolving %s", gitRevision)
	}

	if err := workTree.Checkout(&git.CheckoutOptions{Hash: *hashes}); err != nil {
		return errors.Wrapf(err, "unable to checkout revision: %s", gitRevision)
	}

	err = copyDir(tmpDir, dir)
	if err != nil {
		return fmt.Errorf("failed to copy: %s: %s", dir, err.Error())
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
				Commit: hashes.String(),
			},
		},
	}
	if err := toml.NewEncoder(projectMetadataFile).Encode(projectMd); err != nil {
		return errors.Wrapf(err, "invalid metadata destination '%s/project-metadata.toml' for git repository: %s", metadataDir, gitRevision)
	}

	f.Logger.Printf("Successfully cloned %q @ %q in path %q", gitURL, gitRevision, dir)
	return nil
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dest, srcInfo.Mode())
}

func copyDir(src string, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dest, srcInfo.Mode()); err != nil {
		return err
	}

	fileInfos, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}

	for _, fileInfo := range fileInfos {
		srcPath := path.Join(src, fileInfo.Name())
		destPath := path.Join(dest, fileInfo.Name())

		if fileInfo.IsDir() {
			if err = copyDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
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
