package main

import (
	"flag"
	"os"

	"github.com/buildpack/imgutil/remote"
	"github.com/buildpack/lifecycle"
	"github.com/buildpack/lifecycle/cmd"

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
	tags := flag.Args()

	cmd.Exit(rebase(tags))
}

func rebase(tags []string) error {
	if len(tags) < 1 {
		return cmd.FailCode(cmd.CodeInvalidArgs, "must provide one or more image tags")
	}

	keychain, err := dockercreds.ParseMountedAnnotatedSecrets(buildSecretsDir, dockerCredentials)
	if err != nil {
		return cmd.FailErrCode(err, cmd.CodeInvalidArgs)
	}

	appImage, err := remote.NewImage(tags[0], keychain, remote.FromBaseImage(*lastBuiltImage))
	if err != nil {
		return err
	}

	newBaseImage, err := remote.NewImage(*runImage, keychain, remote.FromBaseImage(*runImage))
	if err != nil {
		return err
	}

	rebaser := lifecycle.Rebaser{
		Logger: cmd.Logger,
	}
	return rebaser.Rebase(appImage, newBaseImage, tags[1:])
}
