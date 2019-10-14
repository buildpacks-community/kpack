package git

import (
	"log"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

type Fetcher struct {
	Logger   *log.Logger
	Keychain GitKeychain
}

func (f Fetcher) Fetch(dir, gitURL, gitRevision string) error {
	resolvedAuth, err := f.Keychain.Resolve(gitURL)
	if err != nil {
		return err
	}

	repo, err := git.PlainInit(dir, false)
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

	err = workTree.Checkout(&git.CheckoutOptions{
		Hash: *hashes,
	})
	if err != nil {
		return errors.Wrapf(err, "unable to checkout revision: %s", gitRevision)
	}

	f.Logger.Printf("Successfully cloned %q @ %q in path %q", gitURL, gitRevision, dir)
	return nil
}
