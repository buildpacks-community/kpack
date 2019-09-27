package main

import (
	"log"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"

	kpackgit "github.com/pivotal/kpack/pkg/git"
)

type GitKeychain interface {
	Resolve(gitUrl string) (kpackgit.Auth, error)
}

func checkoutGitSource(keychain GitKeychain, dir string, logger *log.Logger) {
	resolvedAuth, err := keychain.Resolve(*gitURL)
	if err != nil {
		logger.Fatal(errors.Wrap(err, "error retrieving the authorization"))
	}

	repo, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:               *gitURL,
		Auth:              resolvedAuth.Auth(),
		RemoteName:        "origin",
		Depth:             1,
		RecurseSubmodules: 1,
	})
	if err != nil {
		logger.Fatal(errors.Wrap(err, "unable to fetch git repository"))
	}

	worktree, err := repo.Worktree()
	if err != nil {
		logger.Fatal(errors.Wrap(err, "unable to retrieve working tree"))
	}

	err = worktree.Checkout(&git.CheckoutOptions{
		Hash: plumbing.NewHash(*gitRevision),
	})
	if err != nil {
		logger.Fatal(errors.Wrap(err, "unable to checkout"))
	}

	logger.Printf("Successfully cloned %q @ %q in path %q", *gitURL, *gitRevision, dir)
}
