package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/buildpacks/imgutil/remote"
	"github.com/buildpacks/lifecycle"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
)

const (
	buildSecretsDir = "/var/build-secrets"
)

var (
	runImage       = flag.String("run-image", os.Getenv("RUN_IMAGE"), "The new run image to rebase")
	lastBuiltImage = flag.String("last-built-image", os.Getenv("LAST_BUILT_IMAGE"), "The previous image to rebase")

	dockerCredentials       flaghelpers.CredentialsFlags
	dockerCfgCredentials    flaghelpers.CredentialsFlags
	dockerConfigCredentials flaghelpers.CredentialsFlags
)

func init() {
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCfgCredentials, "dockercfg", "Docker Cfg credentials in the form of the path to the credential")
	flag.Var(&dockerConfigCredentials, "dockerconfig", "Docker Config JSON credentials in the form of the path to the credential")
}

func main() {
	flag.Parse()
	tags := flag.Args()
	logger := log.New(os.Stdout, "", 0)

	cmd.Exit(rebase(tags, logger))
}

func rebase(tags []string, logger *log.Logger) error {
	if len(tags) < 1 {
		return cmd.FailCode(cmd.CodeInvalidArgs, "must provide one or more image tags")
	}

	keychain, err := dockercreds.ParseMountedAnnotatedSecrets(buildSecretsDir, dockerCredentials)
	if err != nil {
		return cmd.FailErrCode(err, cmd.CodeInvalidArgs)
	}

	for _, c := range append(dockerCfgCredentials, dockerConfigCredentials...) {
		credPath := filepath.Join(buildSecretsDir, c)

		dockerCfgCreds, err := dockercreds.ParseDockerPullSecrets(credPath)
		if err != nil {
			return err
		}

		for domain := range dockerCfgCreds {
			logger.Printf("Loading secret for %q from secret %q at location %q", domain, c, credPath)
		}

		keychain, err = keychain.Append(dockerCfgCreds)
		if err != nil {
			return err
		}
	}

	appImage, err := remote.NewImage(tags[0], keychain, remote.FromBaseImage(*lastBuiltImage))
	if err != nil {
		return err
	}

	if !appImage.Found() {
		return errors.Errorf("could not access previous image: %s", *lastBuiltImage)
	}

	newBaseImage, err := remote.NewImage(*runImage, keychain, remote.FromBaseImage(*runImage))
	if err != nil {
		return err
	}

	if !newBaseImage.Found() {
		return errors.Errorf("could not access run image: %s", *runImage)
	}

	rebaser := lifecycle.Rebaser{
		Logger: cmd.Logger,
	}
	return rebaser.Rebase(appImage, newBaseImage, tags[1:])
}
