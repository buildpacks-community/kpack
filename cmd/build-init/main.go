package main

import (
	"flag"
	"log"
	"os"
	"path"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/registry"
)

var (
	platformEnvVars = flag.String("platformEnvVars", os.Getenv("PLATFORM_ENV_VARS"), "a JSON string of build time environment variables formatted as key/value pairs")
	imageTag        = flag.String("imageTag", os.Getenv("IMAGE_TAG"), "tag of image that will get created by the lifecycle")

	gitURL        = flag.String("git-url", os.Getenv("GIT_URL"), "The url of the Git repository to initialize.")
	gitRevision   = flag.String("git-revision", os.Getenv("GIT_REVISION"), "The Git revision to make the repository HEAD.")
	blobURL       = flag.String("blob-url", os.Getenv("BLOB_URL"), "The url of the source code blob.")
	registryImage = flag.String("registry-image", os.Getenv("REGISTRY_IMAGE"), "The registry location of the source code image.")

	gitCredentials    credentialsFlags
	dockerCredentials credentialsFlags
)

func init() {
	flag.Var(&gitCredentials, "basic-git", "Basic authentication for git on the form 'secretname=git.domain.com'")
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker on form 'secretname=git.domain.com'")
}

const (
	secretsHome           = "/builder/home"
	appDir                = "/workspace"
	platformDir           = "/platform"
	buildSecretsDir       = "/var/build-secrets"
	imagePullSecretsDir   = "/imagePullSecrets"
	builderPullSecretsDir = "/builderPullSecrets"
)

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "prepare:", log.Lshortfile)

	creds, err := dockercreds.ParseMountedAnnotatedSecrets(buildSecretsDir, dockerCredentials)
	if err != nil {
		logger.Fatal(err)
	}

	hasWriteAccess, err := dockercreds.HasWriteAccess(creds, *imageTag)
	if err != nil {
		logger.Fatal(err)
	}

	if !hasWriteAccess {
		logger.Fatalf("invalid credentials to build to %s", *imageTag)
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
		gitKeychain, err := git.NewMountedSecretGitKeychain(buildSecretsDir, gitCredentials)
		if err != nil {
			return err
		}

		fetcher := git.Fetcher{
			Logger:   logger,
			Keychain: gitKeychain,
		}
		return fetcher.Fetch(appDir, *gitURL, *gitRevision)
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
			Keychain: authn.NewMultiKeychain(imagePullSecrets, serviceAccountCreds),
		}
		return fetcher.Fetch(appDir, *registryImage)
	default:
		return errors.New("no git url, blob url, or registry image provided")
	}
}
