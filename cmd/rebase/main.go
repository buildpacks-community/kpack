package main

import (
	"flag"
	"log"
	"os"

	"github.com/buildpack/imgutil/remote"
	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/logging"

	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
)

const (
	buildSecretsDir = "/var/build-secrets"
)

var (
	runImage       = flag.String("run-image", os.Getenv("RUN_IMAGE"), "The new run image to rebase")
	lastBuiltImage = flag.String("last-built-image", os.Getenv("LAST_BUILT_IMAGE"), "The previous image to rebase")

	dockerCredentials flaghelpers.CredentialsFlags
)

func init() {
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
}

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "rebase:", log.Lshortfile)

	tags := flag.Args()
	if len(tags) < 1 {
		logger.Fatal("must provide one or more image tags")
	}

	keychain, err := dockercreds.ParseMountedAnnotatedSecrets(buildSecretsDir, dockerCredentials)
	if err != nil {
		logger.Fatal(err)
	}

	appImage, err := remote.NewImage(tags[0], keychain, remote.FromBaseImage(*lastBuiltImage))
	if err != nil {
		logger.Fatal(err)
	}

	newBaseImage, err := remote.NewImage(*runImage, keychain, remote.FromBaseImage(*runImage))
	if err != nil {
		logger.Fatal(err)
	}

	rebaser := lifecycle.Rebaser{
		Logger: logging.New(logger.Writer()),
	}
	err = rebaser.Rebase(appImage, newBaseImage, tags[1:])
	if err != nil {
		logger.Fatal(err)
	}
}
