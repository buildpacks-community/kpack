package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle/platform"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/pkg/errors"
	"github.com/sigstore/cosign/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/cmd/cosign/cli/sign"

	_ "github.com/pivotal/kpack/internal/logrus/fatal"
	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/cosign"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
	"github.com/pivotal/kpack/pkg/notary"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	registrySecretsDir   = "/var/build-secrets"
	reportFilePath       = "/var/report/report.toml"
	notarySecretDir      = "/var/notary/v1"
	cosignSecretLocation = "/var/build-secrets/cosign"
)

var (
	cacheTag                string
	terminationMsgPath      string
	notaryV1URL             string
	dockerCredentials       flaghelpers.CredentialsFlags
	dockerCfgCredentials    flaghelpers.CredentialsFlags
	dockerConfigCredentials flaghelpers.CredentialsFlags
	cosignAnnotations       flaghelpers.CredentialsFlags
	cosignRepositories      flaghelpers.CredentialsFlags
	cosignDockerMediaTypes  flaghelpers.CredentialsFlags
	basicGitCredentials     flaghelpers.CredentialsFlags
	sshGitCredentials       flaghelpers.CredentialsFlags
	logger                  *log.Logger
)

func init() {
	flag.StringVar(&cacheTag, "cache-tag", os.Getenv(buildapi.CacheTagEnvVar), "Tag of image cache")
	flag.StringVar(&terminationMsgPath, "termination-message-path", os.Getenv(buildapi.TerminationMessagePathEnvVar), "Termination path for build metadata")
	flag.StringVar(&notaryV1URL, "notary-v1-url", "", "Notary V1 server url")
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCfgCredentials, "dockercfg", "Docker Cfg credentials in the form of the path to the credential")
	flag.Var(&dockerConfigCredentials, "dockerconfig", "Docker Config JSON credentials in the form of the path to the credential")
	flag.Var(&basicGitCredentials, "basic-git", "Basic authentication for git of the form 'secretname=git.domain.com'")
	flag.Var(&sshGitCredentials, "ssh-git", "SSH authentication for git of the form 'secretname=git.domain.com'")

	flag.Var(&cosignAnnotations, "cosign-annotations", "Cosign custom signing annotations")
	flag.Var(&cosignRepositories, "cosign-repositories", "Cosign signing repository of the form 'secretname=registry.example.com/project'")
	flag.Var(&cosignDockerMediaTypes, "cosign-docker-media-types", "Cosign signing with legacy docker media types of the form 'secretname=1'")
	logger = log.New(os.Stdout, "", 0)
}

func main() {
	flag.Parse()

	var report platform.ExportReport
	_, err := toml.DecodeFile(reportFilePath, &report)
	if err != nil {
		log.Fatal(err, "error decoding report toml file")
	}

	k8sNodeKeychain, err := k8schain.NewNoClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	creds, err := dockercreds.ParseBasicAuthSecrets(registrySecretsDir, dockerCredentials)
	if err != nil {
		log.Fatal(err)
	}

	for _, c := range append(dockerCfgCredentials, dockerConfigCredentials...) {
		credPath := filepath.Join(registrySecretsDir, c)

		dockerConfigCreds, err := dockercreds.ParseDockerConfigSecret(credPath)
		if err != nil {
			log.Fatal(err)
		}

		creds, err = creds.Append(dockerConfigCreds)
		if err != nil {
			log.Fatal(err)
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "error obtaining home directory"))
	}

	err = creds.Save(filepath.Join(homeDir, ".docker", "config.json"))
	if err != nil {
		log.Fatal(errors.Wrapf(err, "error writing docker creds"))
	}

	keychain := authn.NewMultiKeychain(k8sNodeKeychain, creds)

	metadataRetriever := cnb.RemoteMetadataRetriever{
		ImageFetcher: &registry.Client{},
	}

	if len(report.Image.Tags) == 0 {
		log.Fatal("no image found in report")
	}

	builtImageRef := fmt.Sprintf("%s@%s", report.Image.Tags[0], report.Image.Digest)

	buildMetadata, err := metadataRetriever.GetBuildMetadata(builtImageRef, cacheTag, keychain)
	if err != nil {
		log.Fatal(err)
	}

	data, err := cnb.CompressBuildMetadata(buildMetadata)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(terminationMsgPath), 0777); err != nil {
		log.Fatal(err)
	}

	if err := ioutil.WriteFile(terminationMsgPath, data, 0666); err != nil {
		log.Fatal(err)
	}

	if hasCosign() || notaryV1URL != "" {
		if err := signImage(report, keychain); err != nil {
			log.Fatal(err)
		}
	}

	logger.Println("Build successful")
}

func signImage(report platform.ExportReport, keychain authn.Keychain) error {
	if hasCosign() {
		cosignSigner := cosign.NewImageSigner(logger, sign.SignCmd)

		annotations, err := mapKeyValueArgs(cosignAnnotations)
		if err != nil {
			return err
		}

		repositories, err := mapKeyValueArgs(cosignRepositories)
		if err != nil {
			return err
		}

		mediaTypes, err := mapKeyValueArgs(cosignDockerMediaTypes)
		if err != nil {
			return err
		}

		if err := cosignSigner.Sign(
			&options.RootOptions{Timeout: options.DefaultTimeout},
			report,
			cosignSecretLocation,
			annotations,
			repositories,
			mediaTypes); err != nil {
			return errors.Wrap(err, "cosign sign")
		}
	}

	if notaryV1URL != "" {
		signer := notary.ImageSigner{
			Logger:  logger,
			Client:  &registry.Client{},
			Factory: &notary.RemoteRepositoryFactory{},
		}
		if err := signer.Sign(notaryV1URL, notarySecretDir, report, keychain); err != nil {
			return err
		}
	}
	return nil
}

func mapKeyValueArgs(args flaghelpers.CredentialsFlags) (map[string]interface{}, error) {
	overrides := make(map[string]interface{})

	for _, arg := range args {
		splitArg := strings.Split(arg, "=")

		if len(splitArg) != 2 {
			return nil, errors.Errorf("argument not formatted as -arg=key=value: %s", arg)
		}

		key := splitArg[0]
		value := splitArg[1]

		overrides[key] = value
	}

	return overrides, nil
}

func hasCosign() bool {
	_, err := os.Stat(cosignSecretLocation)
	return !os.IsNotExist(err)
}
