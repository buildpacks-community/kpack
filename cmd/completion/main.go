package main

import (
	"flag"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle"
	"github.com/pivotal/kpack/pkg/cosigner"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
	"github.com/pivotal/kpack/pkg/notary"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	registrySecretsDir = "/var/build-secrets"
	reportFilePath     = "/var/report/report.toml"
	notarySecretDir    = "/var/notary/v1"
	buildNumberKey     = "buildNumber"
	buildTimestampKey  = "buildTimestamp"
)

var (
	notaryV1URL             string
	dockerCredentials       flaghelpers.CredentialsFlags
	dockerCfgCredentials    flaghelpers.CredentialsFlags
	dockerConfigCredentials flaghelpers.CredentialsFlags
	cosignAnnotations       flaghelpers.CredentialsFlags
	cosignRepositories      flaghelpers.CredentialsFlags
	cosignDockerMediaTypes  flaghelpers.CredentialsFlags
	logger                  *log.Logger
)

func init() {
	flag.StringVar(&notaryV1URL, "notary-v1-url", "", "Notary V1 server url")
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCfgCredentials, "dockercfg", "Docker Cfg credentials in the form of the path to the credential")
	flag.Var(&dockerConfigCredentials, "dockerconfig", "Docker Config JSON credentials in the form of the path to the credential")
	flag.Var(&cosignAnnotations, "cosign-annotations", "Cosign custom signing annotations")
	flag.Var(&cosignRepositories, "cosign-repositories", "Cosign signing repository of the form 'secretname=registry.example.com/project'")
	flag.Var(&cosignDockerMediaTypes, "cosign-docker-media-types", "Cosign signing with legacy docker media types of the form 'secretname=1'")
	logger = log.New(os.Stdout, "", 0)
}

func main() {
	flag.Parse()

	var report lifecycle.ExportReport
	_, err := toml.DecodeFile(reportFilePath, &report)
	if err != nil {
		logger.Fatalf("toml decode: %v", err)
	}

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

		homeDir, err := os.UserHomeDir()
		if err != nil {
			logger.Fatalf("error obtaining home directory: %v", err)
		}

		err = creds.Save(path.Join(homeDir, ".docker", "config.json"))
		if err != nil {
			logger.Fatalf("error writing docker creds: %v", err)
		}
	}

	cosignSigner := cosigner.ImageSigner{
		Logger: logger,
	}

	annotations := mapKeyValueArgs(cosignAnnotations)
	cosignRepositoryOverrides := mapKeyValueArgs(cosignRepositories)
	cosignDockerMediaTypesOverrides := mapKeyValueArgs(cosignDockerMediaTypes)

	if err := cosignSigner.Sign(report, annotations, cosignRepositoryOverrides, cosignDockerMediaTypesOverrides); err != nil {
		logger.Fatalf("cosignSigner sign: %v\n", err)
	}

	if notaryV1URL != "" {
		signer := notary.ImageSigner{
			Logger:  logger,
			Client:  &registry.Client{},
			Factory: &notary.RemoteRepositoryFactory{},
		}
		if err := signer.Sign(notaryV1URL, notarySecretDir, report, creds); err != nil {
			logger.Fatal(err)
		}
	}

	logger.Println("Build successful")
}

func mapKeyValueArgs(args flaghelpers.CredentialsFlags) map[string]interface{} {
	overrides := make(map[string]interface{})

	for _, arg := range args {
		splitArg := strings.Split(arg, "=")

		if len(splitArg) != 2 {
			logger.Fatalf("argument not formatted as -arg=key=value: %s", arg)
		}

		key := splitArg[0]
		value := splitArg[1]

		overrides[key] = value
	}

	return overrides
}
