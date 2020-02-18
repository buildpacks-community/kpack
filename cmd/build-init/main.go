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

	basicGitCredentials flaghelpers.CredentialsFlags
	sshGitCredentials   flaghelpers.CredentialsFlags
	dockerCredentials   flaghelpers.CredentialsFlags
)

func init() {
	flag.Var(&basicGitCredentials, "basic-git", "Basic authentication for git of the form 'secretname=git.domain.com'")
	flag.Var(&sshGitCredentials, "ssh-git", "SSH authentication for git of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
}

const (
	secretsHome           = "/builder/home"
	appDir                = "/workspace"
	platformDir           = "/platform"
	buildSecretsDir       = "/var/build-secrets"
	imagePullSecretsDir   = "/imagePullSecrets"
	builderPullSecretsDir = "/builderPullSecrets"
	layersDir             = "/alt-layers"
)

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "prepare:", log.Lshortfile)

	creds, err := dockercreds.ParseMountedAnnotatedSecrets(buildSecretsDir, dockerCredentials)
	if err != nil {
		logger.Fatal(err)
	}

	hasImageWriteAccess, err := dockercreds.HasWriteAccess(creds, *imageTag)
	if err != nil {
		logger.Fatal(err)
	}

	if !hasImageWriteAccess {
		logger.Fatalf("invalid credentials to build to %s", *imageTag)
	}

	hasRunImageReadAccess, err := dockercreds.HasReadAccess(creds, *runImage)
	if err != nil {
		logger.Fatal(err)
	}

	if !hasRunImageReadAccess {
		logger.Fatalf("no read access to run image %s", *runImage)
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
		gitKeychain, err := git.NewMountedSecretGitKeychain(buildSecretsDir, basicGitCredentials, sshGitCredentials)
		if err != nil {
			return err
		}

		fetcher := git.Fetcher{
			Logger:   logger,
			Keychain: gitKeychain,
		}
		return fetcher.Fetch(appDir, *gitURL, *gitRevision, layersDir)
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
