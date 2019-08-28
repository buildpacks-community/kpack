package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"os/exec"
)

var (
	gitURL        = flag.String("git-url", os.Getenv("GIT_URL"), "The url of the Git repository to initialize.")
	gitRevision   = flag.String("git-revision", os.Getenv("GIT_REVISION"), "The Git revision to make the repository HEAD.")
	blobURL       = flag.String("blob-url", os.Getenv("BLOB_URL"), "The url of the source code blob.")
	registryImage = flag.String("registry-image", os.Getenv("REGISTRY_IMAGE"), "The registry location of the source code image.")
)

func run(logger *log.Logger, cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output
	if err := c.Run(); err != nil {
		logger.Printf("Error running %v %v: %v\n%v", cmd, args, err, output.String())
	}
}

func runOrFail(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output

	if err := c.Run(); err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "source-init:", log.Lshortfile)

	dir, err := os.Getwd()
	if err != nil {
		logger.Fatal("Failed to get current dir", err)
	}

	if *gitURL != "" {
		checkoutGitSource(dir, logger)
	} else if *blobURL != "" {
		downloadBlob(dir, logger)
	} else if *registryImage != "" {
		fetchImage(dir, logger)
	} else {
		logger.Fatal("no git url, blob url, or registry image provided")
	}
}
