package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/pkg/errors"

	_ "github.com/pivotal/kpack/internal/logrus/fatal"
	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/flaghelpers"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/registry"
)

var (
	imageTag = flag.String("imageTag", os.Getenv("IMAGE_TAG"), "tag of image that will get created by the lifecycle")
	runImage = flag.String("runImage", os.Getenv("RUN_IMAGE"), "The base image from which application images are built.")

	gitURL                  = flag.String("git-url", os.Getenv("GIT_URL"), "The url of the Git repository to initialize.")
	gitRevision             = flag.String("git-revision", os.Getenv("GIT_REVISION"), "The Git revision to make the repository HEAD.")
	gitInitializeSubmodules = flag.Bool("git-initialize-submodules", getenvBool("GIT_INITIALIZE_SUBMODULES"), "Initialize submodules during git clone")
	blobURL                 = flag.String("blob-url", os.Getenv("BLOB_URL"), "The url of the source code blob.")
	blobAuth                = flag.Bool("blob-auth", getenvBool("BLOB_AUTH"), "If authentication should be used for blobs")
	stripComponents         = flag.Int("strip-components", getenvInt("BLOB_STRIP_COMPONENTS", 0), "The number of directory components to strip from the blobs content when extracting.")
	registryImage           = flag.String("registry-image", os.Getenv("REGISTRY_IMAGE"), "The registry location of the source code image.")
	hostName                = flag.String("dns-probe-hostname", os.Getenv("DNS_PROBE_HOSTNAME"), "hostname to dns poll")
	sourceSubPath           = flag.String("source-sub-path", os.Getenv("SOURCE_SUB_PATH"), "the subpath inside the source directory that will be the buildpack workspace")
	buildChanges            = flag.String("build-changes", os.Getenv("BUILD_CHANGES"), "JSON string of build changes and their reason")
	descriptorPath          = flag.String("project-descriptor-path", os.Getenv("PROJECT_DESCRIPTOR_PATH"), "path to project descriptor file")

	builderImage = flag.String("builder-image", os.Getenv("BUILDER_IMAGE"), "The builder image used to build the application")
	builderName  = flag.String("builder-name", os.Getenv("BUILDER_NAME"), "The builder name provided during creation")
	builderKind  = flag.String("builder-kind", os.Getenv("BUILDER_KIND"), "The builder kind")

	basicGitCredentials     flaghelpers.CredentialsFlags
	sshGitCredentials       flaghelpers.CredentialsFlags
	blobCredentials         flaghelpers.CredentialsFlags
	basicDockerCredentials  flaghelpers.CredentialsFlags
	dockerCfgCredentials    flaghelpers.CredentialsFlags
	dockerConfigCredentials flaghelpers.CredentialsFlags
	imagePullSecrets        flaghelpers.CredentialsFlags

	sshTrustUnknownHosts = flag.Bool("insecure-ssh-trust-unknown-hosts", flaghelpers.GetEnvBool("INSECURE_SSH_TRUST_UNKNOWN_HOSTS", true), "Trust unknown hosts when using SSH authentication")
)

func init() {
	flag.Var(&basicGitCredentials, "basic-git", "Basic authentication for git of the form 'secretname=git.domain.com'")
	flag.Var(&sshGitCredentials, "ssh-git", "SSH authentication for git of the form 'secretname=git.domain.com'")
	flag.Var(&blobCredentials, "blob", "Authentication for blob of the form 'secretname=git.domain.com'")
	flag.Var(&basicDockerCredentials, "basic-docker", "Basic authentication for docker of the form 'secretname=git.domain.com'")
	flag.Var(&dockerCfgCredentials, "dockercfg", "Docker Cfg credentials in the form of the path to the credential")
	flag.Var(&dockerConfigCredentials, "dockerconfig", "Docker Config JSON credentials in the form of the path to the credential")
	flag.Var(&imagePullSecrets, "imagepull", "Builder Image pull credentials in the form of the path to the credential")
}

const (
	secretsHome                  = "/builder/home"
	appDir                       = "/workspace"
	platformDir                  = "/platform"
	buildSecretsDir              = "/var/build-secrets"
	registrySourcePullSecretsDir = "/registrySourcePullSecrets"
	projectMetadataDir           = "/projectMetadata" // place to write project-metadata.toml which gets exported to image label by the lifecycle
	networkWaitLauncherDir       = "/networkWait"
	networkWaitLauncherBinary    = "network-wait-launcher.exe"
)

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "", 0)

	err := prepareForWindows(*hostName)
	if err != nil {
		logger.Fatal(err)
	}

	if err := buildchange.Log(logger, *buildChanges); err != nil {
		logger.Println(err)
	}

	logger.Println("Loading registry credentials from service account secrets")

	logLoadingSecrets(logger, basicDockerCredentials)
	creds, err := dockercreds.ParseBasicAuthSecrets(buildSecretsDir, basicDockerCredentials)
	if err != nil {
		logger.Fatal(err)
	}

	for _, c := range append(dockerCfgCredentials, dockerConfigCredentials...) {
		credPath := filepath.Join(buildSecretsDir, c)

		dockerCfgCreds, err := dockercreds.ParseDockerConfigSecret(credPath)
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

	if len(creds) == 0 {
		logger.Println("No registry credentials were loaded from service account secrets")
	}

	logger.Println("Loading cluster credential helpers")
	k8sNodeKeychain, err := k8schain.NewNoClient(context.Background())
	if err != nil {
		logger.Fatal(err)
	}

	err = dockercreds.VerifyWriteAccess(authn.NewMultiKeychain(creds, k8sNodeKeychain), *imageTag)
	if err != nil {
		logger.Fatal(errors.Wrapf(err, "Error verifying write access to %q", *imageTag))
	}

	for _, c := range imagePullSecrets {
		credPath := filepath.Join(buildSecretsDir, c)

		imagePullCreds, err := dockercreds.ParseDockerConfigSecret(credPath)
		if err != nil {
			logger.Fatal(err)
		}

		creds, err = creds.Append(imagePullCreds)
		if err != nil {
			logger.Fatalf("error appending image pull creds %s", err)
		}
	}

	keychain := authn.NewMultiKeychain(creds, k8sNodeKeychain)
	err = dockercreds.VerifyReadAccess(keychain, *runImage)
	if err != nil {
		logger.Fatal(errors.Wrapf(err, "Error verifying read access to run image %q", *runImage))
	}

	err = fetchSource(logger, keychain)
	if err != nil {
		logger.Fatal(err)
	}

	err = cnb.ProcessProjectDescriptor(filepath.Join(appDir, *sourceSubPath), *descriptorPath, platformDir, logger)
	if err != nil {
		logger.Fatalf("error while processing the project descriptor: %s", err)
	}

	if *builderImage != "" && *builderName != "" && *builderKind != "" {
		logger.Printf("Builder:\n Image: %s \n Name: %s \n Kind: %s ", *builderImage,
			*builderName, *builderKind)
	}

	err = cnb.SetupPlatformEnvVars(platformDir)
	if err != nil {
		logger.Fatalf("error setting up platform env vars %s", err)
	}

	err = creds.Save(path.Join(secretsHome, ".docker", "config.json"))
	if err != nil {
		logger.Fatalf("error writing docker creds %s", err)
	}
}

func prepareForWindows(hostname string) error {
	if runtime.GOOS != "windows" {
		return nil
	}

	executablePath, err := os.Executable()
	if err != nil {
		return err
	}

	err = copyFile(filepath.Join(filepath.Dir(executablePath), networkWaitLauncherBinary), filepath.Join(networkWaitLauncherDir, networkWaitLauncherBinary))
	if err != nil {
		return err
	}

	waitForDns(hostname)

	return nil
}

func fetchSource(logger *log.Logger, keychain authn.Keychain) error {
	switch {
	case *gitURL != "":
		logLoadingSecrets(logger, basicGitCredentials, sshGitCredentials)

		gitKeychain, err := git.NewMountedSecretGitKeychain(buildSecretsDir, basicGitCredentials, sshGitCredentials, *sshTrustUnknownHosts)
		if err != nil {
			return err
		}

		var initializeSubmodules bool
		if gitInitializeSubmodules != nil {
			initializeSubmodules = *gitInitializeSubmodules
		}

		fetcher := git.Fetcher{
			Logger:               logger,
			Keychain:             gitKeychain,
			InitializeSubmodules: initializeSubmodules,
		}
		return fetcher.Fetch(appDir, *gitURL, *gitRevision, projectMetadataDir)
	case *blobURL != "":
		var (
			blobKeychain blob.Keychain
			err          error
		)
		if *blobAuth {
			if len(blobCredentials) == 0 {
				logger.Println("Loading blob credentials from helpers")
				blobKeychain = blob.DefaultKeychain
			} else {
				logger.Println("Loading blob credentials from service account secrets")
				logLoadingSecrets(logger, blobCredentials)
				blobKeychain, err = blob.NewMountedSecretBlobKeychain(buildSecretsDir, blobCredentials)
				if err != nil {
					return err
				}
			}
		}

		fetcher := blob.Fetcher{
			Logger:   logger,
			Keychain: blobKeychain,
		}
		return fetcher.Fetch(appDir, *blobURL, *stripComponents, projectMetadataDir)
	case *registryImage != "":
		registrySourcePullSecrets, err := dockercreds.ParseDockerConfigSecret(registrySourcePullSecretsDir)
		if err != nil {
			return err
		}

		fetcher := registry.Fetcher{
			Logger:   logger,
			Client:   &registry.Client{},
			Keychain: authn.NewMultiKeychain(registrySourcePullSecrets, keychain),
		}
		return fetcher.Fetch(appDir, *registryImage, projectMetadataDir)
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

func waitForDns(hostname string) {
	timeoutChan := time.After(10 * time.Second)
	tickerChan := time.NewTicker(time.Second)
	defer tickerChan.Stop()

	for {
		select {
		case <-timeoutChan:
			return
		case <-tickerChan.C:
			if _, err := net.LookupIP(hostname); err == nil {
				return
			}
		}
	}
}

func copyFile(src, dest string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err = io.Copy(destFile, srcFile); err != nil {
		return err
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dest, srcInfo.Mode())
}

func getenvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	atoi, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return atoi
}

func getenvBool(key string) bool {
	value := os.Getenv(key)
	b, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return b
}
