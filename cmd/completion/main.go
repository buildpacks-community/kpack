package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
	"github.com/pivotal/kpack/pkg/notary"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	registrySecretsDir = "/var/build-secrets"
	reportFilePath     = "/var/report/report.toml"
	notarySecretDir    = "/var/notary/v1"
)

var (
	notaryV1URL             string
	caCertFilePath          string
	dockerCredentials       flaghelpers.CredentialsFlags
	dockerCfgCredentials    flaghelpers.CredentialsFlags
	dockerConfigCredentials flaghelpers.CredentialsFlags
	logger                  *log.Logger
)

func init() {
	flag.StringVar(&notaryV1URL, "notary-v1-url", "", "Notary V1 server url")
	flag.StringVar(&caCertFilePath, "ca-cert", "", "CA certificate path")
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCfgCredentials, "dockercfg", "Docker Cfg credentials in the form of the path to the credential")
	flag.Var(&dockerConfigCredentials, "dockerconfig", "Docker Config JSON credentials in the form of the path to the credential")

	logger = log.New(os.Stdout, "", 0)
}

func main() {
	flag.Parse()

	if notaryV1URL != "" {
		creds, err := dockercreds.ParseMountedAnnotatedSecrets(registrySecretsDir, dockerCredentials)
		if err != nil {
			logger.Fatal(err)
		}

		for _, c := range append(dockerCfgCredentials, dockerConfigCredentials...) {
			credPath := filepath.Join(registrySecretsDir, c)

			dockerCfgCreds, err := dockercreds.ParseDockerPullSecrets(credPath)
			if err != nil {
				logger.Fatal(err)
			}

			for domain := range dockerCfgCreds {
				logger.Printf("Loading secret for %q from secret %q at location %q", domain, c, credPath)
			}

			creds, err = creds.Append(dockerCfgCreds)
			if err != nil {
				logger.Fatal(err)
			}
		}

		signer := notary.ImageSigner{
			Logger:  logger,
			Client:  &registry.Client{},
			Factory: &notary.RemoteRepositoryFactory{},
		}
		if err := signer.Sign(notaryV1URL, notarySecretDir, reportFilePath, caCertFilePath, creds); err != nil {
			logger.Fatal(err)
		}
	}

	logger.Println("Build successful")
}
