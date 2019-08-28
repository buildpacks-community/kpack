package main

import (
	"flag"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/registry"
)

var (
	builder         = flag.String("builder", os.Getenv("BUILDER"), "the builder to initialize the env for a build")
	platformEnvVars = flag.String("platformEnvVars", os.Getenv("PLATFORM_ENV_VARS"), "a JSON string of build time environment variables formatted as key/value pairs")
	imageTag        = flag.String("imageTag", os.Getenv("IMAGE_TAG"), "tag of image that will get created by the lifecycle")
)

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "prepare:", log.Lshortfile)

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	hasWriteAccess, err := registry.HasWriteAccess(*imageTag)
	if err != nil {
		log.Fatal(err)
	}

	if !hasWriteAccess {
		log.Fatalf("invalid credentials to build to %s", *imageTag)
	}

	err = os.MkdirAll(filepath.Join(usr.HomeDir, ".docker"), os.ModePerm)
	if err != nil {
		logger.Fatal(err)
	}

	dockerConfigs, err := dockercreds.ParseDockerPullSecrets("/builderPullSecrets")
	if err != nil {
		log.Fatal(err)
	}

	err = dockerConfigs.AppendCredsToDockerConfig("/builder/home/.docker/config.json")
	if err != nil {
		log.Fatal(err)
	}

	remoteImageFactory := &registry.ImageFactory{
		KeychainFactory: defaultKeychainFactory{},
	}

	filePermissionSetup := &cnb.FilePermissionSetup{
		RemoteImageFactory: remoteImageFactory,
		Chowner:            realOs{},
	}
	err = filePermissionSetup.Setup(
		*builder,
		"/builder/home", "/layersDir", "/cache", "/workspace",
	)
	if err != nil {
		logger.Fatalf("error setting up permissions %s", err)
	}

	err = cnb.SetupPlatformEnvVars("/platform", *platformEnvVars)
	if err != nil {
		logger.Fatalf("error setting up platform env vars %s", err)
	}
}

type defaultKeychainFactory struct {
}

func (defaultKeychainFactory) KeychainForImageRef(registry.ImageRef) authn.Keychain {
	return authn.DefaultKeychain
}

type realOs struct {
}

func (realOs) Chown(volume string, uid, gid int) error {
	return os.Chown(volume, uid, gid)
}

func fileExists(file string, logger *log.Logger) bool {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		logger.Fatal(err.Error())
	}

	return true
}
