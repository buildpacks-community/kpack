package main

import (
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/imgutil/remote"
	"github.com/buildpacks/lifecycle"
	"github.com/buildpacks/lifecycle/api"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/pkg/errors"

	_ "github.com/pivotal/kpack/internal/logrus/fatal"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
)

const (
	buildSecretsDir = "/var/build-secrets"
)

var (
	runImage       = flag.String("run-image", os.Getenv("RUN_IMAGE"), "The new run image to rebase")
	lastBuiltImage = flag.String("last-built-image", os.Getenv("LAST_BUILT_IMAGE"), "The previous image to rebase")
	buildChanges   = flag.String("build-changes", os.Getenv("BUILD_CHANGES"), "JSON string of build changes and their reason")
	reportFilePath = flag.String("report", os.Getenv("REPORT_FILE_PATH"), "The location at which to write the report.toml")

	basicDockerCredentials  flaghelpers.CredentialsFlags
	dockerCfgCredentials    flaghelpers.CredentialsFlags
	dockerConfigCredentials flaghelpers.CredentialsFlags
	imagePullSecrets        flaghelpers.CredentialsFlags
)

func init() {
	flag.Var(&basicDockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCfgCredentials, "dockercfg", "Docker Cfg credentials in the form of the path to the credential")
	flag.Var(&dockerConfigCredentials, "dockerconfig", "Docker Config JSON credentials in the form of the path to the credential")
	flag.Var(&imagePullSecrets, "imagepull", "Builder Image pull credentials in the form of the path to the credential")
}

func main() {
	flag.Parse()
	tags := flag.Args()
	logger := log.New(os.Stdout, "", 0)

	if err := buildchange.Log(logger, *buildChanges); err != nil {
		logger.Println(err)
	}

	cmd.Exit(rebase(tags, logger))
}

func rebase(tags []string, logger *log.Logger) error {
	if len(tags) < 1 {
		return cmd.FailCode(cmd.CodeInvalidArgs, "must provide one or more image tags")
	}

	logger.Println("Loading cluster credential helpers")
	k8sNodeKeychain, err := k8schain.NewNoClient(context.Background())
	if err != nil {
		return err
	}

	logLoadingSecrets(logger, basicDockerCredentials)
	creds, err := dockercreds.ParseBasicAuthSecrets(buildSecretsDir, basicDockerCredentials)
	if err != nil {
		return cmd.FailErrCode(err, cmd.CodeInvalidArgs)
	}

	for _, c := range combine(dockerCfgCredentials, dockerConfigCredentials, imagePullSecrets) {
		credPath := filepath.Join(buildSecretsDir, c)

		dockerCfgCreds, err := dockercreds.ParseDockerConfigSecret(credPath)
		if err != nil {
			return err
		}

		for domain := range dockerCfgCreds {
			logger.Printf("Loading secret for %q from secret %q at location %q", domain, c, credPath)
		}

		creds, err = creds.Append(dockerCfgCreds)
		if err != nil {
			return err
		}
	}

	keychain := authn.NewMultiKeychain(creds, k8sNodeKeychain)

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
		Logger:      cmd.DefaultLogger,
		PlatformAPI: api.MustParse("0.9"),
	}
	report, err := rebaser.Rebase(appImage, newBaseImage, tags[1:])
	if err != nil {
		return err
	}

	if *reportFilePath == "" {
		return nil
	}

	buf := &bytes.Buffer{}
	err = toml.NewEncoder(buf).Encode(report)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(*reportFilePath, buf.Bytes(), 0777)
}

func logLoadingSecrets(logger *log.Logger, secretsSlices ...[]string) {
	for _, secretsSlice := range secretsSlices {
		for _, secret := range secretsSlice {
			splitSecret := strings.Split(secret, "=")
			if len(splitSecret) == 2 {
				secretName := splitSecret[0]
				domain := splitSecret[1]
				logger.Printf("Loading secrets for %q from secret %q", domain, secretName)
			}
		}
	}
}

func combine(credentials ...[]string) []string {
	var combinded []string
	for _, creds := range credentials {
		combinded = append(combinded, creds...)
	}
	return combinded
}
