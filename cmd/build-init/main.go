package main

import (
	"flag"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/pivotal/build-service-system/pkg/cnb"
	"github.com/pivotal/build-service-system/pkg/registry"
)

var (
	builder         = flag.String("builder", os.Getenv("BUILDER"), "the builder to initialize the env for a build")
	platformEnvVars = flag.String("platformEnvVars", os.Getenv("PLATFORM_ENV_VARS"), "a JSON string of build time environment variables formatted as key/value pairs")
)

func main() {
	flag.Parse()

	remoteImageFactory := &registry.ImageFactory{
		KeychainFactory: defaultKeychainFactory{},
	}

	filePermissionSetup := &cnb.FilePermissionSetup{
		RemoteImageFactory: remoteImageFactory,
		Chowner:            realOs{},
	}
	err := filePermissionSetup.Setup(
		*builder,
		"/builder/home", "/layersDir", "/cache", "/workspace",
	)
	if err != nil {
		log.Fatalf("error setting up permissions %s", err)
	}

	err = cnb.SetupPlatformEnvVars("/platform", *platformEnvVars)
	if err != nil {
		log.Fatalf("error setting up platform env vars %s", err)
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
