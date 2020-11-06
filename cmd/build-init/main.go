package main

import (
	"flag"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/registry"
)

var (
	platformEnvVars = flag.String("platformEnvVars", os.Getenv("PLATFORM_ENV_VARS"), "a JSON string of build time environment variables formatted as key/value pairs")
	imageTag        = flag.String("imageTag", os.Getenv("IMAGE_TAG"), "tag of image that will get created by the lifecycle")
	runImage        = flag.String("runImage", os.Getenv("RUN_IMAGE"), "run image that the build the image on")

	gitURL        = flag.String("git-url", os.Getenv("GIT_URL"), "The url of the Git repository to initialize.")
	gitRevision   = flag.String("git-revision", os.Getenv("GIT_REVISION"), "The Git revision to make the repository HEAD.")
	blobURL       = flag.String("blob-url", os.Getenv("BLOB_URL"), "The url of the source code blob.")
	registryImage = flag.String("registry-image", os.Getenv("REGISTRY_IMAGE"), "The registry location of the source code image.")

	basicGitCredentials     flaghelpers.CredentialsFlags
	sshGitCredentials       flaghelpers.CredentialsFlags
	dockerCredentials       flaghelpers.CredentialsFlags
	dockerCfgCredentials    flaghelpers.CredentialsFlags
	dockerConfigCredentials flaghelpers.CredentialsFlags
)

func init() {
	flag.Var(&basicGitCredentials, "basic-git", "Basic authentication for git of the form 'secretname=git.domain.com'")
	flag.Var(&sshGitCredentials, "ssh-git", "SSH authentication for git of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCfgCredentials, "dockercfg", "Docker Cfg credentials in the form of the path to the credential")
	flag.Var(&dockerConfigCredentials, "dockerconfig", "Docker Config JSON credentials in the form of the path to the credential")
}

const (
	secretsHome           = "/builder/home"
	appDir                = "/workspace"
	platformDir           = "/platform"
	buildSecretsDir       = "/var/build-secrets"
	imagePullSecretsDir   = "/imagePullSecrets"
	builderPullSecretsDir = "/builderPullSecrets"
	projectMetadataDir    = "/projectMetadata"
)

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "", 0)

	logLoadingSecrets(logger, dockerCredentials)
	creds, err := dockercreds.ParseMountedAnnotatedSecrets(buildSecretsDir, dockerCredentials)
	if err != nil {
		logger.Fatal(err)
	}

	for _, c := range append(dockerCfgCredentials, dockerConfigCredentials...) {
		credPath := filepath.Join(buildSecretsDir, c)

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

	err = dockercreds.VerifyWriteAccess(creds, *imageTag)
	if err != nil {
		logger.Fatal(errors.Wrapf(err, "Error verifying write access to %q", *imageTag))
	}

	err = dockercreds.VerifyReadAccess(creds, *runImage)
	if err != nil {
		logger.Fatal(errors.Wrapf(err, "Error verifying read access to run image %q", *runImage))
	}

	err = fetchSource(logger, creds)
	if err != nil {
		logger.Fatal(err)
	}

	err = cnb.SetupPlatformEnvVars(platformDir, *platformEnvVars)
	if err != nil {
		logger.Fatalf("error setting up platform env vars %s", err)
	}

	builderCreds, err := dockercreds.ParseDockerPullSecrets(builderPullSecretsDir)
	if err != nil {
		logger.Fatal(err)
	}

	dockerCreds, err := creds.Append(builderCreds)
	if err != nil {
		logger.Fatalf("error appending builder creds %s", err)
	}

	err = dockerCreds.Save(path.Join(secretsHome, ".docker", "config.json"))
	if err != nil {
		logger.Fatalf("error writing docker creds %s", err)
	}
}

func fetchSource(logger *log.Logger, serviceAccountCreds dockercreds.DockerCreds) error {
	switch {
	case *gitURL != "":
		logLoadingSecrets(logger, basicGitCredentials, sshGitCredentials)

		gitKeychain, err := git.NewMountedSecretGitKeychain(buildSecretsDir, basicGitCredentials, sshGitCredentials)
		if err != nil {
			return err
		}

		fetcher := git.Fetcher{
			Logger:   logger,
			Keychain: gitKeychain,
		}
		return fetcher.Fetch(appDir, *gitURL, *gitRevision, projectMetadataDir)
	case *blobURL != "":
		fetcher := blob.Fetcher{
			Logger: logger,
		}
		return fetcher.Fetch(appDir, *blobURL)
	case *registryImage != "":
		imagePullSecrets, err := dockercreds.ParseDockerPullSecrets(imagePullSecretsDir)
		if err != nil {
			return err
		}

		fetcher := registry.Fetcher{
			Logger:   logger,
			Client:   &registry.Client{},
			Keychain: authn.NewMultiKeychain(imagePullSecrets, serviceAccountCreds),
		}
		return fetcher.Fetch(appDir, *registryImage)
	default:
		return errors.New("no git url, blob url, or registry image provided")
	}
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
