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

	hasWriteAccess, err := dockercreds.HasWriteAccess(*imageTag)
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

	builderCreds, err := dockercreds.ParseDockerPullSecrets("/builderPullSecrets")
	if err != nil {
		log.Fatal(err)
	}

	err = builderCreds.AppendToDockerConfig("/builder/home/.docker/config.json")
	if err != nil {
		log.Fatal(err)
	}

	remoteImageFactory := &registry.ImageFactory{
		KeychainFactory: keychainFactory{builderCreds},
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

type keychainFactory struct {
	keychain authn.Keychain
}

func (k keychainFactory) KeychainForImageRef(registry.ImageRef) authn.Keychain {
	return k.keychain
}

type realOs struct {
}

func (realOs) Chown(volume string, uid, gid int) error {
	return os.Chown(volume, uid, gid)
}
