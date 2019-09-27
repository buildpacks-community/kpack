package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/pivotal/kpack/pkg/git"
)

type gitCredentialsFlags []string

func (i *gitCredentialsFlags) String() string {
	return "my string representation"
}

func (i *gitCredentialsFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var (
	gitURL        = flag.String("git-url", os.Getenv("GIT_URL"), "The url of the Git repository to initialize.")
	gitRevision   = flag.String("git-revision", os.Getenv("GIT_REVISION"), "The Git revision to make the repository HEAD.")
	gitCredentials gitCredentialsFlags
	blobURL       = flag.String("blob-url", os.Getenv("BLOB_URL"), "The url of the source code blob.")
	registryImage = flag.String("registry-image", os.Getenv("REGISTRY_IMAGE"), "The registry location of the source code image.")
)

func main() {
	flag.Var(&gitCredentials, "basic-git", "Basic authentication for git on the for 'secretname=git.domain.com'")
	if creds, ok := os.LookupEnv("BASIC_GIT"); ok {
		for _, gitCred := range strings.Split(creds, ",") {
			gitCredentials = append(gitCredentials, gitCred)
		}
	}
	flag.Parse()

	logger := log.New(os.Stdout, "source-init:", log.Lshortfile)

	dir, err := os.Getwd()
	if err != nil {
		logger.Fatal("Failed to get current dir", err)
	}

	keychain := git.NewGitKeychain(gitCredentials, &git.VolumeSecretReader{})

	if *gitURL != "" {
		checkoutGitSource(keychain, dir, logger)
	} else if *blobURL != "" {
		downloadBlob(dir, logger)
	} else if *registryImage != "" {
		fetchImage(dir, logger)
	} else {
		logger.Fatal("no git url, blob url, or registry image provided")
	}
}
