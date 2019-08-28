package main

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
)

func checkoutGitSource(dir string, logger *log.Logger) {
	usr, err := user.Current() // The user should be root to be able to read .git-credentials and .gitconfig
	if err != nil {
		log.Fatal(err)
	}

	symlinks := []string{".ssh", ".git-credentials", ".gitconfig"}
	for _, path := range symlinks {
		err = os.Symlink("/builder/home/"+path, filepath.Join(usr.HomeDir, path))
		if err != nil {
			logger.Fatalf("Unexpected error creating symlink: %v", err)
		}
	}

	run(logger, "git", "init")
	run(logger, "git", "remote", "add", "origin", *gitURL)

	err = runOrFail("git", "fetch", "--depth=1", "--recurse-submodules=yes", "origin", *gitRevision)
	if err != nil {
		run(logger, "git", "pull", "--recurse-submodules=yes", "origin")
		err = runOrFail("git", "checkout", *gitRevision)
		if err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		run(logger, "git", "reset", "--hard", "FETCH_HEAD")
	}
	logger.Printf("Successfully cloned %q @ %q in path %q", *gitURL, *gitRevision, dir)
}
