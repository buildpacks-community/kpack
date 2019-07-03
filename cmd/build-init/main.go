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
	builder = flag.String("builder", os.Getenv("BUILDER"), "the builder to initialize the env for a build")
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
	err := filePermissionSetup.Setup(*builder,
		"/builder/home", "/layersDir", "/cache", "/workspace")
	if err != nil {
		log.Fatalf("error setting up permissions %s", err)
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
